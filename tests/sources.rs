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
    let mock = axum::Router::new()
        .route(
            "/rest/v2/projects",
            axum::routing::get(|| async { axum::Json(json!([])) }),
        )
        .route(
            "/rest/v2/tasks",
            axum::routing::get(|| async {
                axum::Json(json!([{"id":"task-1","content":"Remote task","priority":3}]))
            }),
        );
    let mock_listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let mock_root = format!("http://{}", mock_listener.local_addr().unwrap());
    let mock_server = tokio::spawn(async move { axum::serve(mock_listener, mock).await.unwrap() });
    let response = client
        .patch(format!("{root}/api/sources/{source_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"baseUrl":mock_root}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let source: Value = response.json().await.unwrap();
    assert_eq!(source["connected"], true);
    let response = client
        .post(format!("{root}/api/sources/{source_id}/sync"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let result: Value = response.json().await.unwrap();
    assert_eq!(result["upserted"], 1);
    let title: String =
        sqlx::query_scalar("SELECT title FROM items WHERE source_id=$1 AND external_key='task-1'")
            .bind(Uuid::parse_str(source_id).unwrap())
            .fetch_one(&pool)
            .await
            .unwrap();
    assert_eq!(title, "Remote task");
    mock_server.abort();
    let response = client
        .delete(format!("{root}/api/sources/{source_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let jira_mock = axum::Router::new()
        .route("/rest/api/2/myself", axum::routing::get(|| async { axum::Json(json!({"name":"test"})) }))
        .route("/rest/api/2/search", axum::routing::post(|| async {
            axum::Json(json!({"issues":[{"key":"WB-1","fields":{"summary":"Jira task","priority":{"name":"High"}}}],"total":1}))
        }));
    let jira_listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let jira_root = format!("http://{}", jira_listener.local_addr().unwrap());
    let jira_server =
        tokio::spawn(async move { axum::serve(jira_listener, jira_mock).await.unwrap() });
    let response = client.post(format!("{root}/api/sources"))
        .header("Cookie", &cookie)
        .json(&json!({"kind":"jira","name":"Jira test","projectId":project_id,"baseUrl":jira_root,"email":"test@example.com","token":"jira-token"}))
        .send().await.unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let jira: Value = response.json().await.unwrap();
    assert_eq!(jira["connected"], true);
    let jira_id = jira["id"].as_str().unwrap();
    let response = client
        .post(format!("{root}/api/sources/{jira_id}/sync"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert_eq!(response.json::<Value>().await.unwrap()["upserted"], 1);
    let jira_uuid = Uuid::parse_str(jira_id).unwrap();
    let item_id: Uuid =
        sqlx::query_scalar("SELECT id FROM items WHERE source_id=$1 AND external_key='WB-1'")
            .bind(jira_uuid)
            .fetch_one(&pool)
            .await
            .unwrap();
    sqlx::query("UPDATE items SET status='in_progress' WHERE id=$1")
        .bind(item_id)
        .execute(&pool)
        .await
        .unwrap();
    let response = client
        .post(format!("{root}/api/sources/{jira_id}/sync"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert_eq!(response.json::<Value>().await.unwrap()["upserted"], 1);
    let status: String = sqlx::query_scalar("SELECT status FROM items WHERE id=$1")
        .bind(item_id)
        .fetch_one(&pool)
        .await
        .unwrap();
    assert_eq!(status, "in_progress");
    let response = client
        .delete(format!("{root}/api/sources/{jira_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    jira_server.abort();
    let response = client
        .delete(format!("{root}/api/projects/{project_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    server.abort();
}
