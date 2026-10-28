use serde_json::{json, Value};
use sqlx::PgPool;
use std::sync::Arc;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn project_routes_keep_response_shape() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
    let app = router(AppState {
        pool,
        auth,
        cors_origin: "http://127.0.0.1:5173".to_owned(),
        token_box: Arc::new(
            TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
                .unwrap(),
        ),
    });
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = listener.local_addr().unwrap();
    let server = tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    let client = reqwest::Client::new();
    let root = format!("http://{address}");
    let response = client
        .post(format!("{root}/api/projects"))
        .header("Cookie", format!("wb_session={token}"))
        .json(&json!({"name":"  Project parity  ","targetHoursDay":2.0}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let created: Value = response.json().await.unwrap();
    assert_eq!(created["name"], "Project parity");
    assert_eq!(created["targetHoursWeek"], 14.0);
    assert_eq!(created["people"], json!([]));
    assert_eq!(created["companies"], json!([]));
    let id = created["id"].as_str().unwrap();
    let response = client
        .get(format!("{root}/api/projects/{id}"))
        .header("Cookie", format!("wb_session={token}"))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert_eq!(response.json::<Value>().await.unwrap()["id"], id);
    let response = client
        .patch(format!("{root}/api/projects/{id}"))
        .header("Cookie", format!("wb_session={token}"))
        .json(&json!({"archived":true}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert!(response.json::<Value>().await.unwrap()["archivedAt"].is_string());
    let response = client
        .post(format!("{root}/api/projects/{id}/notes"))
        .header("Cookie", format!("wb_session={token}"))
        .json(&json!({"body":"  project note  "}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let note: Value = response.json().await.unwrap();
    assert_eq!(note["body"], "project note");
    let response = client
        .delete(format!(
            "{root}/api/projects/{id}/notes/{}",
            note["id"].as_str().unwrap()
        ))
        .header("Cookie", format!("wb_session={token}"))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let response = client
        .delete(format!("{root}/api/projects/{id}"))
        .header("Cookie", format!("wb_session={token}"))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let response = client
        .post(format!("{root}/api/notes"))
        .header("Cookie", format!("wb_session={token}"))
        .json(&json!({"body":"  personal note  "}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let note: Value = response.json().await.unwrap();
    assert_eq!(note["body"], "personal note");
    assert!(note["itemId"].is_null());
    let note_id = note["id"].as_str().unwrap();
    let response = client
        .patch(format!("{root}/api/notes/{note_id}"))
        .header("Cookie", format!("wb_session={token}"))
        .json(&json!({"body":"changed"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert_eq!(response.json::<Value>().await.unwrap()["body"], "changed");
    let response = client
        .delete(format!("{root}/api/notes/{note_id}"))
        .header("Cookie", format!("wb_session={token}"))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    server.abort();
}
