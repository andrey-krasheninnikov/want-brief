use axum::routing::get;
use serde_json::{json, Value};
use sqlx::PgPool;
use std::sync::Arc;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn syncs_remote_details_without_changing_local_status_or_repeating_comments() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
    let token_box = Arc::new(
        TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff").unwrap(),
    );
    let mock = axum::Router::new()
        .route("/rest/v2/tasks/remote-1", get(|| async {
            axum::Json(json!({"id":"remote-1","content":"Updated title","description":"Updated description","is_completed":true}))
        }))
        .route("/rest/v2/comments", get(|| async {
            axum::Json(json!([{"id":"comment-1","content":"Remote comment","posted_at":"2026-01-01T12:00:00Z"}]))
        }));
    let mock_listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let mock_url = format!("http://{}", mock_listener.local_addr().unwrap());
    let mock_server = tokio::spawn(async move { axum::serve(mock_listener, mock).await.unwrap() });
    let source_id = Uuid::new_v4();
    let item_id = Uuid::new_v4();
    let project_id: Uuid = sqlx::query_scalar("SELECT id FROM projects WHERE name='Work' LIMIT 1")
        .fetch_one(&pool)
        .await
        .unwrap();
    let sealed = token_box.seal("secret-token").unwrap();
    sqlx::query("INSERT INTO sources (id,kind,name,base_url,token_sealed,query_filter,project_id) VALUES ($1,'todoist','Sync test',$2,$3,'',$4)")
        .bind(source_id).bind(&mock_url).bind(sealed).bind(project_id).execute(&pool).await.unwrap();
    sqlx::query("INSERT INTO items (id,source_id,external_key,title,status,kind,project_id,description) VALUES ($1,$2,'remote-1','Old title','in_progress','task',$3,'Old description')")
        .bind(item_id).bind(source_id).bind(project_id).execute(&pool).await.unwrap();
    let app = router(AppState {
        pool: pool.clone(),
        auth,
        cors_origin: String::new(),
        token_box,
    });
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let root = format!("http://{}", listener.local_addr().unwrap());
    let server = tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    let client = reqwest::Client::new();
    let cookie = format!("wb_session={token}");
    for _ in 0..2 {
        let response = client
            .post(format!("{root}/api/items/{item_id}/sync"))
            .header("Cookie", &cookie)
            .send()
            .await
            .unwrap();
        assert_eq!(response.status(), reqwest::StatusCode::OK);
        let item: Value = response.json().await.unwrap();
        assert_eq!(item["title"], "Updated title");
        assert_eq!(item["status"], "in_progress");
        assert_eq!(item["externalStatus"], "Completed");
        assert_eq!(item["description"], "Updated description");
    }
    let notes: i64 = sqlx::query_scalar("SELECT count(*) FROM item_notes WHERE item_id=$1")
        .bind(item_id)
        .fetch_one(&pool)
        .await
        .unwrap();
    assert_eq!(notes, 1);
    let response = client
        .post(format!("{root}/api/items/sync-active"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let counts: Value = response.json().await.unwrap();
    assert!(counts["synced"].as_i64().unwrap() >= 1);
    sqlx::query("DELETE FROM sources WHERE id=$1")
        .bind(source_id)
        .execute(&pool)
        .await
        .unwrap();
    server.abort();
    mock_server.abort();
}
