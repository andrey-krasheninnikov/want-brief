use reqwest::{Client, Method, RequestBuilder, StatusCode};
use serde_json::Value;
use sqlx::{PgPool, Postgres, Transaction};
use std::sync::Arc;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};
use want_brief::workspace;

pub struct Account {
    pub id: Uuid,
    pub personal: Uuid,
    cookie: String,
}

pub async fn space(
    tx: &mut Transaction<'_, Postgres>,
    owner: Uuid,
    kind: &str,
    plan: &str,
) -> Uuid {
    let id = sqlx::query_scalar(
        "INSERT INTO workspaces(owner_id,kind,name,plan) VALUES($1,$2,$2,$3) RETURNING id",
    )
    .bind(owner)
    .bind(kind)
    .bind(plan)
    .fetch_one(&mut **tx)
    .await
    .unwrap();
    sqlx::query("INSERT INTO workspace_members(workspace_id,user_id,role) VALUES($1,$2,'owner')")
        .bind(id)
        .bind(owner)
        .execute(&mut **tx)
        .await
        .unwrap();
    workspace::seed(tx, id, owner).await.unwrap();
    id
}

pub async fn account(tx: &mut Transaction<'_, Postgres>) -> Account {
    let id = Uuid::new_v4();
    let token = Uuid::new_v4().to_string();
    sqlx::query("INSERT INTO users(id,email,password_hash) VALUES($1,$2,'unusable')")
        .bind(id)
        .bind(format!("{id}@example.test"))
        .execute(&mut **tx)
        .await
        .unwrap();
    sqlx::query("INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,encode(sha256(convert_to($2,'UTF8')),'hex'),now() + interval '1 hour')")
        .bind(id).bind(&token).execute(&mut **tx).await.unwrap();
    Account {
        id,
        personal: space(tx, id, "personal", "free").await,
        cookie: format!("wb_session={token}"),
    }
}

pub struct Api {
    client: Client,
    root: String,
}

impl Api {
    pub fn request(
        &self,
        method: Method,
        account: &Account,
        space: Uuid,
        path: &str,
    ) -> RequestBuilder {
        self.client
            .request(method, format!("{}/api/{path}", self.root))
            .header("Cookie", &account.cookie)
            .header("X-Workspace-Id", space.to_string())
    }
}

pub async fn value(request: RequestBuilder, expected: StatusCode) -> Value {
    let response = request.send().await.unwrap();
    let status = response.status();
    let body = response.text().await.unwrap();
    assert_eq!(status, expected, "{body}");
    serde_json::from_str(&body).unwrap()
}

pub async fn start(control: &PgPool) -> (Api, tokio::task::JoinHandle<()>) {
    let app = router(AppState {
        pool: control.clone(),
        auth: Auth::new(control.clone()),
        cors_origin: String::new(),
        token_box: Arc::new(TokenBox::new(&"11".repeat(32)).unwrap()),
    });
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let api = Api {
        client: Client::new(),
        root: format!("http://{}", listener.local_addr().unwrap()),
    };
    let server = tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    (api, server)
}
