use serde_json::{json, Value};
use sqlx::PgPool;
use std::sync::Arc;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn source_routes_store_tokens_without_exposing_them() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
    let box_key = Arc::new(
        TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff").unwrap(),
    );
    let app = router(AppState {
        pool: pool.clone(),
        auth,
        cors_origin: String::new(),
        token_box: box_key.clone(),
    });
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let root = format!("http://{}", listener.local_addr().unwrap());
    let server = tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    let client = reqwest::Client::new();
    let cookie = format!("wb_session={token}");
    let response = client
        .post(format!("{root}/api/projects"))
        .header("Cookie", &cookie)
        .json(&json!({"name":"Source test project"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let project: Value = response.json().await.unwrap();
    let project_id = project["id"].as_str().unwrap();
    let response = client.post(format!("{root}/api/sources"))
        .header("Cookie", &cookie)
        .json(&json!({"kind":"todoist","name":"Source test","projectId":project_id,"baseUrl":"http://127.0.0.1:1","query":"today"}))
        .send().await.unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let source: Value = response.json().await.unwrap();
    assert_eq!(source["hasToken"], false);
    assert!(source.get("tokenSealed").is_none());
    let source_id = source["id"].as_str().unwrap();
    let response = client
        .patch(format!("{root}/api/sources/{source_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"token":"sample-token"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let source: Value = response.json().await.unwrap();
    assert_eq!(source["hasToken"], true);
    assert!(source.get("tokenSealed").is_none());
    assert!(!source.to_string().contains("sample-token"));
    let sealed: String = sqlx::query_scalar("SELECT token_sealed FROM sources WHERE id=$1")
        .bind(Uuid::parse_str(source_id).unwrap())
        .fetch_one(&pool)
        .await
        .unwrap();
    assert_ne!(sealed, "sample-token");
    assert_eq!(box_key.open(&sealed).unwrap(), "sample-token");
    let response = client
        .delete(format!("{root}/api/sources/{source_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let response = client
        .delete(format!("{root}/api/projects/{project_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    server.abort();
}
