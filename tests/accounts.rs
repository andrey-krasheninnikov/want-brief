use reqwest::{Client, Method, Response, StatusCode};
use serde_json::{json, Value};
use sqlx::postgres::PgPoolOptions;
use sqlx::PgPool;
use std::sync::Arc;
use tokio::io::{AsyncBufReadExt, AsyncWriteExt, BufReader};
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::mpsc;
use tokio::task::JoinHandle;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};
use want_brief::mail::Mailer;

#[tokio::test]
async fn malformed_confirmation_is_rejected_without_opening_a_session() {
    let pool = PgPoolOptions::new()
        .connect_lazy("postgres://localhost/unused")
        .unwrap();
    let app = router(AppState {
        auth: Auth::new(pool.clone()),
        pool,
        cors_origin: "http://localhost".to_owned(),
        token_box: Arc::new(TokenBox::new(&"11".repeat(32)).unwrap()),
    });
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr = listener.local_addr().unwrap();
    let server = tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    let response = Client::new()
        .post(format!("http://{addr}/api/auth/confirm"))
        .json(&json!({"token":"bad","password":"a long sample phrase"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), StatusCode::BAD_REQUEST);
    assert!(response.headers().get("set-cookie").is_none());
    server.abort();
}

struct Server {
    api: String,
    client: Client,
    mail: mpsc::Receiver<String>,
    tasks: [JoinHandle<()>; 2],
}

impl Drop for Server {
    fn drop(&mut self) {
        for task in &self.tasks {
            task.abort();
        }
    }
}

async fn accept_mail(stream: TcpStream, sender: mpsc::Sender<String>) -> std::io::Result<()> {
    let (reader, mut writer) = stream.into_split();
    let mut lines = BufReader::new(reader).lines();
    writer.write_all(b"220 localhost ESMTP\r\n").await?;
    while let Some(command) = lines.next_line().await? {
        if command.starts_with("EHLO ") {
            writer
                .write_all(b"250-localhost\r\n250 8BITMIME\r\n")
                .await?;
        } else if command == "DATA" {
            writer.write_all(b"354 Send message\r\n").await?;
            let mut message = String::new();
            while let Some(line) = lines.next_line().await? {
                if line == "." {
                    break;
                }
                message.push_str(&line);
                message.push('\n');
            }
            sender.send(message).await.unwrap();
            writer.write_all(b"250 Accepted\r\n").await?;
        } else if command == "QUIT" {
            writer.write_all(b"221 Goodbye\r\n").await?;
            break;
        } else {
            writer.write_all(b"250 OK\r\n").await?;
        }
    }
    Ok(())
}

impl Server {
    async fn start(url: &str) -> Self {
        let smtp = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let port = smtp.local_addr().unwrap().port();
        let (send, mail) = mpsc::channel(8);
        let smtp_task = tokio::spawn(async move {
            while let Ok((stream, _)) = smtp.accept().await {
                accept_mail(stream, send.clone()).await.unwrap();
            }
        });
        let pool = PgPool::connect(url).await.unwrap();
        let transport = Mailer::new(
            "127.0.0.1",
            Some(port),
            "none",
            "brief@example.invalid",
            None,
        )
        .unwrap();
        let auth = Auth::configured(
            pool.clone(),
            "https://brief.example.invalid",
            Some(transport),
        )
        .unwrap();
        let app = router(AppState {
            pool,
            auth,
            cors_origin: "https://brief.example.invalid".into(),
            token_box: Arc::new(TokenBox::new(&"11".repeat(32)).unwrap()),
        });
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let api = format!("http://{}", listener.local_addr().unwrap());
        let http_task = tokio::spawn(async move {
            axum::serve(
                listener,
                app.into_make_service_with_connect_info::<std::net::SocketAddr>(),
            )
            .await
            .unwrap();
        });
        Self {
            api,
            client: Client::new(),
            mail,
            tasks: [smtp_task, http_task],
        }
    }

    async fn request(
        &self,
        method: Method,
        path: &str,
        cookie: Option<&str>,
        body: Option<Value>,
    ) -> Response {
        let mut req = self
            .client
            .request(method, format!("{}{path}", self.api))
            .header("origin", "https://brief.example.invalid");
        if let Some(cookie) = cookie {
            req = req.header("cookie", cookie);
        }
        if let Some(body) = body {
            req = req.json(&body);
        }
        req.send().await.unwrap()
    }

    async fn post(&self, path: &str, cookie: Option<&str>, body: Value) -> Response {
        self.request(Method::POST, path, cookie, Some(body)).await
    }

    async fn token(&mut self, email: &str, action: &str) -> String {
        let mail = tokio::time::timeout(std::time::Duration::from_secs(4), self.mail.recv())
            .await
            .unwrap()
            .unwrap();
        assert!(
            mail.lines().any(|line| line == format!("To: {email}")),
            "wrong email recipient"
        );
        let decoded = mail.replace("=\n", "").replace("=3D", "=");
        let prefix = format!("https://brief.example.invalid/login#{action}=");
        let token = decoded
            .lines()
            .find_map(|line| line.strip_prefix(&prefix))
            .expect("missing email action");
        assert!(
            token.len() == 64 && token.bytes().all(|b| b.is_ascii_hexdigit()),
            "invalid email token"
        );
        token.into()
    }

    async fn create_user(&mut self, email: &str, password: &str) -> String {
        assert_eq!(
            self.post(
                "/api/auth/register",
                None,
                json!({"email":email.to_ascii_uppercase()})
            )
            .await
            .status(),
            StatusCode::OK
        );
        assert_eq!(
            self.post(
                "/api/login",
                None,
                json!({"email":email,"password":password})
            )
            .await
            .status(),
            StatusCode::UNAUTHORIZED
        );
        let token = self.token(email, "verify").await;
        assert_eq!(
            self.post(
                "/api/auth/confirm",
                None,
                json!({"token":token,"password":password})
            )
            .await
            .status(),
            StatusCode::OK
        );
        assert_eq!(
            self.post(
                "/api/auth/confirm",
                None,
                json!({"token":token,"password":password})
            )
            .await
            .status(),
            StatusCode::BAD_REQUEST
        );
        self.login(Some(email), password).await
    }

    async fn login(&self, email: Option<&str>, password: &str) -> String {
        let response = self
            .post(
                "/api/login",
                None,
                json!({"email":email,"password":password}),
            )
            .await;
        assert_eq!(response.status(), StatusCode::OK);
        let cookie = response
            .headers()
            .get("set-cookie")
            .unwrap()
            .to_str()
            .unwrap();
        for attribute in ["HttpOnly", "Secure", "SameSite=Lax"] {
            assert!(cookie.contains(attribute));
        }
        cookie.split(';').next().unwrap().into()
    }
}

fn test_database() -> String {
    let url = std::env::var("TEST_DATABASE_URL").expect("isolated TEST_DATABASE_URL");
    let db = reqwest::Url::parse(&url).unwrap();
    assert!(db
        .path()
        .trim_start_matches('/')
        .starts_with("wantbrief_issue92_"));
    url
}

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn confirmed_accounts_invite_by_address_and_recover_by_email() {
    let db = test_database();
    let mut server = Server::start(&db).await;
    let suffix = Uuid::new_v4().simple();
    let owner_email = format!("owner-{suffix}@example.invalid");
    let member_email = format!("member-{suffix}@example.invalid");
    let password = "initial test passphrase";
    let full_length_password = "x".repeat(72);
    let owner = server
        .create_user(&owner_email, &full_length_password)
        .await;
    assert_eq!(
        server
            .post(
                "/api/login",
                None,
                json!({"email":owner_email,"password":format!("{full_length_password}y")})
            )
            .await
            .status(),
        StatusCode::UNAUTHORIZED
    );
    let member = server.create_user(&member_email, password).await;
    let team = server
        .post(
            "/api/workspaces",
            Some(&owner),
            json!({"name":"Mail test team"}),
        )
        .await;
    assert_eq!(team.status(), StatusCode::CREATED);
    let team_id = team.json::<Value>().await.unwrap()["id"]
        .as_str()
        .unwrap()
        .to_owned();
    let invites = format!("/api/workspaces/{team_id}/invitations");
    let members = format!("/api/workspaces/{team_id}/members");
    let body = json!({"email":member_email});
    assert_eq!(
        server
            .post(&invites, Some(&owner), body.clone())
            .await
            .status(),
        StatusCode::FORBIDDEN
    );
    let activated = std::process::Command::new(env!("CARGO_BIN_EXE_want-brief"))
        .args(["set-plan", &team_id, "paid"])
        .env("DATABASE_URL", &db)
        .output()
        .unwrap();
    assert!(activated.status.success(), "operator tariff command failed");
    assert_eq!(
        server
            .request(Method::GET, &members, Some(&member), None)
            .await
            .status(),
        StatusCode::NOT_FOUND
    );
    assert_eq!(
        server
            .post(&invites, Some(&owner), body.clone())
            .await
            .status(),
        StatusCode::OK
    );
    let revoked = server.token(&member_email, "invite").await;
    let list = server
        .request(Method::GET, &invites, Some(&owner), None)
        .await
        .json::<Value>()
        .await
        .unwrap();
    let invitation_id = list[0]["id"].as_str().unwrap();
    assert_eq!(
        server
            .request(
                Method::DELETE,
                &format!("{invites}/{invitation_id}"),
                Some(&owner),
                None
            )
            .await
            .status(),
        StatusCode::NO_CONTENT
    );
    assert_eq!(
        server
            .post(
                "/api/invitations/accept",
                Some(&member),
                json!({"token":revoked})
            )
            .await
            .status(),
        StatusCode::BAD_REQUEST
    );
    assert_eq!(
        server.post(&invites, Some(&owner), body).await.status(),
        StatusCode::OK
    );
    let token = server.token(&member_email, "invite").await;
    assert_eq!(
        server
            .post(
                "/api/invitations/accept",
                Some(&owner),
                json!({"token":token})
            )
            .await
            .status(),
        StatusCode::BAD_REQUEST
    );
    let accepted = server
        .post(
            "/api/invitations/accept",
            Some(&member),
            json!({"token":token}),
        )
        .await;
    assert_eq!(accepted.status(), StatusCode::OK);
    assert_eq!(
        accepted.json::<Value>().await.unwrap()["workspaceId"],
        team_id
    );
    assert_eq!(
        server
            .post(
                "/api/invitations/accept",
                Some(&member),
                json!({"token":token})
            )
            .await
            .status(),
        StatusCode::BAD_REQUEST
    );
    let roster = server
        .request(Method::GET, &members, Some(&member), None)
        .await
        .json::<Vec<Value>>()
        .await
        .unwrap();
    assert_eq!(roster.len(), 2);
    assert!(roster
        .iter()
        .all(|m| m["label"].as_str().is_some_and(|name| !name.contains('@'))));
    assert_eq!(
        server
            .request(Method::GET, &invites, Some(&member), None)
            .await
            .status(),
        StatusCode::NOT_FOUND
    );

    let old = server.login(Some(&member_email), password).await;
    let sessions = server
        .request(Method::GET, "/api/account/sessions", Some(&member), None)
        .await
        .json::<Vec<Value>>()
        .await
        .unwrap();
    assert_eq!(sessions.len(), 2);
    let other = sessions
        .into_iter()
        .find(|s| s["current"] == false)
        .unwrap();
    let path = format!("/api/account/sessions/{}", other["id"].as_str().unwrap());
    assert_eq!(
        server
            .request(Method::DELETE, &path, Some(&owner), None)
            .await
            .status(),
        StatusCode::NOT_FOUND
    );
    assert_eq!(
        server
            .request(Method::DELETE, &path, Some(&member), None)
            .await
            .status(),
        StatusCode::NO_CONTENT
    );
    assert_eq!(
        server
            .request(Method::GET, "/api/me", Some(&old), None)
            .await
            .status(),
        StatusCode::UNAUTHORIZED
    );
    assert_eq!(
        server
            .post("/api/auth/forgot", None, json!({"email":member_email}))
            .await
            .status(),
        StatusCode::OK
    );
    let reset = server.token(&member_email, "reset").await;
    let new_password = "updated sample passphrase";
    assert_eq!(
        server
            .post(
                "/api/auth/reset",
                None,
                json!({"token":reset,"password":new_password})
            )
            .await
            .status(),
        StatusCode::OK
    );
    assert_eq!(
        server
            .post(
                "/api/auth/reset",
                None,
                json!({"token":reset,"password":password})
            )
            .await
            .status(),
        StatusCode::BAD_REQUEST
    );
    assert_eq!(
        server
            .request(Method::GET, "/api/me", Some(&member), None)
            .await
            .status(),
        StatusCode::UNAUTHORIZED
    );
    assert_eq!(
        server
            .post(
                "/api/login",
                None,
                json!({"email":member_email,"password":password})
            )
            .await
            .status(),
        StatusCode::UNAUTHORIZED
    );
    let recovered = server.login(Some(&member_email), new_password).await;
    assert_eq!(
        server
            .request(Method::GET, &members, Some(&recovered), None)
            .await
            .status(),
        StatusCode::OK
    );
    assert_eq!(
        server
            .request(Method::GET, "/api/me", Some(&owner), None)
            .await
            .status(),
        StatusCode::OK
    );
}

#[tokio::test]
#[ignore = "requires an isolated single-owner TEST_DATABASE_URL"]
async fn legacy_owner_keeps_workspace_and_password_after_binding_email() {
    let db = test_database();
    let pool = PgPool::connect(&db).await.unwrap();
    let owner_id: Uuid =
        sqlx::query_scalar("SELECT id FROM users WHERE is_legacy AND email IS NULL")
            .fetch_one(&pool)
            .await
            .expect("migrated single-owner test fixture");
    let email = format!("bind-{}@example.invalid", Uuid::new_v4().simple());
    let bound_email = email.clone();
    let result = tokio::spawn(async move {
        let mut server = Server::start(&db).await;
        let password = "sample-password";
        let cookie = server.login(None, password).await;
        let before = server
            .request(Method::GET, "/api/me", Some(&cookie), None)
            .await
            .json::<Value>()
            .await
            .unwrap();
        assert_eq!(before["workspace"]["plan"], "paid");
        assert!(before["user"]["email"].is_null());
        let workspace_id = before["workspace"]["id"].as_str().unwrap();
        let downgrade = std::process::Command::new(env!("CARGO_BIN_EXE_want-brief"))
            .args(["set-plan", workspace_id, "free"])
            .env("DATABASE_URL", &db)
            .output()
            .unwrap();
        assert!(!downgrade.status.success());
        assert_eq!(
            server
                .post(
                    "/api/account/email",
                    Some(&cookie),
                    json!({"email":bound_email,"password":"wrong"})
                )
                .await
                .status(),
            StatusCode::UNAUTHORIZED
        );
        assert_eq!(
            server
                .post(
                    "/api/account/email",
                    Some(&cookie),
                    json!({"email":bound_email,"password":password})
                )
                .await
                .status(),
            StatusCode::OK
        );
        let token = server.token(&bound_email, "bind").await;
        // Binding is effective only after the owner opens the link.
        let second = server.login(None, password).await;
        assert_eq!(
            server
                .post("/api/auth/confirm", None, json!({"token":token}))
                .await
                .status(),
            StatusCode::OK
        );
        assert_eq!(
            server
                .post("/api/auth/confirm", None, json!({"token":token}))
                .await
                .status(),
            StatusCode::BAD_REQUEST
        );
        for previous in [cookie, second] {
            assert_eq!(
                server
                    .request(Method::GET, "/api/me", Some(&previous), None)
                    .await
                    .status(),
                StatusCode::UNAUTHORIZED
            );
        }
        assert_eq!(
            server
                .post("/api/login", None, json!({"password":password}))
                .await
                .status(),
            StatusCode::UNAUTHORIZED
        );
        let current = server.login(Some(&bound_email), password).await;
        let after = server
            .request(Method::GET, "/api/me", Some(&current), None)
            .await
            .json::<Value>()
            .await
            .unwrap();
        assert_eq!(after["user"]["id"], before["user"]["id"]);
        assert_eq!(after["user"]["email"], bound_email);
        assert_eq!(after["workspace"], before["workspace"]);
        assert_eq!(after["workspaces"], before["workspaces"]);
        assert_eq!(
            server
                .post("/api/logout", Some(&current), json!({}))
                .await
                .status(),
            StatusCode::OK
        );
    })
    .await;
    // Restore the shared test fixture even if an HTTP assertion failed.
    sqlx::query("UPDATE users SET email=NULL, credential_version=credential_version+1 WHERE id=$1 AND email=$2 AND is_legacy")
        .bind(owner_id).bind(email).execute(&pool).await.unwrap();
    result.unwrap();
}

#[test]
fn public_mail_links_require_https_outside_loopback() {
    use want_brief::mail::app_url;
    assert!(app_url("http://brief.example.org").is_err());
    assert!(app_url("https://username@brief.example.org").is_err());
    assert!(app_url("http://127.0.0.1:5173").is_ok());
    assert!(Mailer::new(
        "smtp.example.org",
        Some(25),
        "none",
        "test@example.invalid",
        None
    )
    .is_err());
}
