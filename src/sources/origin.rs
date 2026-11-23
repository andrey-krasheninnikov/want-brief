use super::Source;
use reqwest::{redirect::Policy, Client, Url};

const INVALID_URL: &str = "invalid source address";
const DENIED_ORIGIN: &str = "source origin is not enabled for this installation";
const VERIFY_TLS: &str = "TLS verification is required for this workspace";

pub(super) fn denied(message: &str) -> bool {
    matches!(message, INVALID_URL | DENIED_ORIGIN | VERIFY_TLS)
}

fn base_url(raw: &str) -> Result<Url, &'static str> {
    let url = Url::parse(raw).map_err(|_| INVALID_URL)?;
    if !matches!(url.scheme(), "https" | "http")
        || url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
        || url.query().is_some()
        || url.fragment().is_some()
    {
        return Err(INVALID_URL);
    }
    Ok(url)
}

fn jira_cloud(url: &Url) -> bool {
    url.host_str()
        .and_then(|host| host.strip_suffix(".atlassian.net"))
        .is_some_and(|site| !site.is_empty() && !site.contains('.'))
}

fn allowed(
    kind: &str,
    raw: &str,
    insecure_tls: bool,
    legacy_owner: bool,
    origins: &str,
) -> Result<Url, &'static str> {
    let url = base_url(raw)?;
    if legacy_owner {
        return Ok(url);
    }
    if insecure_tls {
        return Err(VERIFY_TLS);
    }
    let standard = url.scheme() == "https"
        && url.port().is_none()
        && match kind {
            "jira" => jira_cloud(&url),
            "todoist" => url.host_str() == Some("api.todoist.com"),
            _ => false,
        };
    if standard
        || origins
            .split(',')
            .filter_map(|entry| base_url(entry.trim()).ok())
            .any(|entry| entry.path() == "/" && entry.origin() == url.origin())
    {
        Ok(url)
    } else {
        Err(DENIED_ORIGIN)
    }
}

pub(super) fn validate(source: &Source) -> Result<Url, &'static str> {
    let scope = crate::workspace::current().ok_or(DENIED_ORIGIN)?;
    let legacy_owner = scope.legacy_paid && scope.kind == "personal" && scope.role == "owner";
    let origins = std::env::var("SOURCE_ALLOWED_ORIGINS").unwrap_or_default();
    allowed(
        &source.kind,
        &source.base_url,
        source.insecure_tls,
        legacy_owner,
        &origins,
    )
}

pub(crate) struct SourceHttp {
    pub(crate) client: Client,
    pub(crate) cloud: bool,
    base: Url,
}

impl SourceHttp {
    pub(crate) fn new(source: &Source) -> Result<Self, String> {
        let base = validate(source)?;
        let redirects = if source.kind == "jira" {
            Policy::none()
        } else {
            same_origin_redirects(&base)
        };
        let client = Client::builder()
            .timeout(std::time::Duration::from_secs(25))
            .redirect(redirects)
            .danger_accept_invalid_certs(source.kind == "jira" && source.insecure_tls)
            .build()
            .map_err(|err| err.to_string())?;
        Ok(Self {
            client,
            cloud: jira_cloud(&base),
            base,
        })
    }

    pub(crate) fn endpoint(&self, path: &[&str]) -> Url {
        let mut url = self.base.clone();
        url.path_segments_mut()
            .expect("HTTP URLs support path segments")
            .pop_if_empty()
            .extend(path);
        url
    }
}

fn same_origin_redirects(base: &Url) -> Policy {
    let origin = base.origin();
    Policy::custom(move |attempt| {
        let next = attempt.url();
        if next.origin() == origin
            && next.username().is_empty()
            && next.password().is_none()
            && attempt.previous().len() < 10
        {
            attempt.follow()
        } else {
            attempt.stop()
        }
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_provider_origins_and_operator_origins_only() {
        assert!(allowed("jira", "https://project.atlassian.net", false, false, "").is_ok());
        assert!(allowed("todoist", "https://api.todoist.com", false, false, "").is_ok());
        for raw in [
            "http://api.todoist.com",
            "https://api.todoist.com:8443",
            "https://api.todoist.com.example.test",
            "https://api.todoist.com/?token=test",
        ] {
            assert!(allowed("todoist", raw, false, false, "").is_err());
        }
        let configured = "https://tasks.example.test, https://work.example.test:8443";
        assert!(allowed(
            "jira",
            "https://work.example.test:8443/jira",
            false,
            false,
            configured
        )
        .is_ok());
        assert!(allowed(
            "jira",
            "https://work.example.test/jira",
            false,
            false,
            configured
        )
        .is_err());
        assert!(allowed(
            "jira",
            "https://work.example.test:8443/jira",
            true,
            false,
            configured
        )
        .is_err());
        assert!(allowed("jira", "http://work.example.test/jira", true, true, "").is_ok());
    }

    #[tokio::test]
    async fn redirects_stay_on_the_configured_origin() {
        use axum::{response::Redirect, routing::get, Router};
        use std::sync::{
            atomic::{AtomicUsize, Ordering},
            Arc,
        };
        let target = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let target_url = format!("http://{}/done", target.local_addr().unwrap());
        let hits = Arc::new(AtomicUsize::new(0));
        let observed = hits.clone();
        let target_server = tokio::spawn(async move {
            let app = Router::new().route(
                "/done",
                get(move || async move {
                    observed.fetch_add(1, Ordering::SeqCst);
                    "done"
                }),
            );
            axum::serve(target, app).await.unwrap();
        });
        let origin = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let base = Url::parse(&format!("http://{}", origin.local_addr().unwrap())).unwrap();
        let origin_server = tokio::spawn(async move {
            let app = Router::new()
                .route("/same", get(|| async { Redirect::temporary("/ok") }))
                .route("/ok", get(|| async { "ok" }))
                .route(
                    "/other",
                    get(move || async move { Redirect::temporary(&target_url) }),
                );
            axum::serve(origin, app).await.unwrap();
        });
        let client = Client::builder()
            .redirect(same_origin_redirects(&base))
            .build()
            .unwrap();
        let same = client.get(base.join("same").unwrap()).send().await.unwrap();
        assert_eq!(same.text().await.unwrap(), "ok");
        let other = client
            .get(base.join("other").unwrap())
            .send()
            .await
            .unwrap();
        assert!(other.status().is_redirection());
        assert_eq!(hits.load(Ordering::SeqCst), 0);
        origin_server.abort();
        target_server.abort();
    }
}
