use anyhow::{bail, Context, Result};
use lettre::message::Mailbox;
use lettre::transport::smtp::authentication::Credentials;
use lettre::{AsyncSmtpTransport, AsyncTransport, Message, Tokio1Executor};
use reqwest::Url;
use std::env;
use std::net::IpAddr;
use std::time::Duration;

#[derive(Clone)]
pub struct Mailer {
    transport: AsyncSmtpTransport<Tokio1Executor>,
    from: Mailbox,
}

fn local_host(host: &str) -> bool {
    host.eq_ignore_ascii_case("localhost")
        || host
            .trim_matches(['[', ']'])
            .parse::<IpAddr>()
            .is_ok_and(|ip| ip.is_loopback())
}

pub fn app_url(address: &str) -> Result<Url> {
    let url = Url::parse(address).context("invalid PUBLIC_APP_URL")?;
    let host = url.host_str().context("PUBLIC_APP_URL requires a host")?;
    if !(url.scheme() == "https" || (url.scheme() == "http" && local_host(host)))
        || !url.username().is_empty()
        || url.password().is_some()
        || url.query().is_some()
        || url.fragment().is_some()
        || url.path() != "/"
    {
        bail!("PUBLIC_APP_URL must be an HTTPS origin (HTTP is allowed on loopback)");
    }
    Ok(url)
}

impl Mailer {
    pub fn from_env() -> Result<Option<Self>> {
        let Some(host) = env::var("SMTP_HOST")
            .ok()
            .filter(|host| !host.trim().is_empty())
        else {
            return Ok(None);
        };
        let port = env::var("SMTP_PORT")
            .ok()
            .map(|p| p.parse())
            .transpose()
            .context("invalid SMTP_PORT")?;
        let from = env::var("SMTP_FROM").context("SMTP_FROM is required")?;
        let mode = env::var("SMTP_TLS").unwrap_or_else(|_| "starttls".into());
        let credentials = match (
            env::var("SMTP_USERNAME").ok(),
            env::var("SMTP_PASSWORD").ok(),
        ) {
            (Some(user), Some(password)) if !user.is_empty() && !password.is_empty() => {
                Some((user, password))
            }
            (None, None) => None,
            _ => bail!("configure both SMTP_USERNAME and SMTP_PASSWORD, or neither"),
        };
        Self::new(&host, port, &mode, &from, credentials).map(Some)
    }

    pub fn new(
        host: &str,
        port: Option<u16>,
        tls: &str,
        from: &str,
        credentials: Option<(String, String)>,
    ) -> Result<Self> {
        let mut transport = match tls {
            "starttls" => AsyncSmtpTransport::<Tokio1Executor>::starttls_relay(host)?,
            "tls" => AsyncSmtpTransport::<Tokio1Executor>::relay(host)?,
            "none" if local_host(host) && credentials.is_none() => AsyncSmtpTransport::<Tokio1Executor>::builder_dangerous(host),
            _ => bail!("SMTP_TLS must be tls or starttls; none is allowed only on loopback without credentials"),
        };
        if let Some(port) = port {
            transport = transport.port(port);
        }
        if let Some((user, password)) = credentials {
            transport = transport.credentials(Credentials::new(user, password));
        }
        Ok(Self {
            transport: transport.timeout(Some(Duration::from_secs(10))).build(),
            from: from.parse().context("invalid SMTP_FROM")?,
        })
    }

    pub(crate) async fn send_link(
        &self,
        app: &Url,
        email: &str,
        action: &str,
        token: &str,
    ) -> Result<()> {
        let (subject, instruction) = match action {
            "verify" => (
                "Confirm your Want Brief email",
                "Choose a password to create your account.",
            ),
            "bind" => (
                "Confirm your Want Brief email",
                "Confirm this address for your existing account.",
            ),
            "reset" => (
                "Reset your Want Brief password",
                "Choose a new password for your account.",
            ),
            "absent" => (
                "Your Want Brief password reset request",
                "This address has no Want Brief account. You can register at the link below.",
            ),
            "invite" => (
                "Your Want Brief invitation",
                "Sign in with this email address to accept your workspace invitation.",
            ),
            _ => bail!("unknown mail action"),
        };
        let mut link = app.join("/login")?;
        if !token.is_empty() {
            link.set_fragment(Some(&format!("{action}={token}")));
        }
        let message = Message::builder()
            .from(self.from.clone())
            .to(email.parse()?)
            .subject(subject)
            .body(format!(
                "{instruction}\n\n{link}\n\nIf you did not expect this email, you can ignore it."
            ))?;
        self.transport
            .send(message)
            .await
            .context("could not send account email")?;
        Ok(())
    }
}
