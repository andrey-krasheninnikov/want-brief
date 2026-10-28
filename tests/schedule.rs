use chrono::{Duration, Utc};
use serde_json::{json, Value};
use sqlx::types::Json as DbJson;
use sqlx::PgPool;
use std::sync::Arc;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn schedule_saves_blocks_and_compares_a_scenario() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
    let previous: Option<(chrono::DateTime<Utc>, DbJson<Value>)> =
        sqlx::query_as("SELECT taken_at,blocks FROM schedule_snapshots WHERE kind='work'")
            .fetch_optional(&pool)
            .await
            .unwrap();
    let app = router(AppState {
        pool: pool.clone(),
        auth,
        cors_origin: String::new(),
        token_box: Arc::new(
            TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
                .unwrap(),
        ),
    });
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let root = format!("http://{}", listener.local_addr().unwrap());
    let server = tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    let client = reqwest::Client::new();
    let cookie = format!("wb_session={token}");
    let item: Value = client
        .post(format!("{root}/api/items"))
        .header("Cookie", &cookie)
        .json(&json!({"title":"Schedule check","kind":"task"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    let item_id = item["id"].as_str().unwrap();
    let now = Utc::now();
    let from = now.to_rfc3339();
    let to = (now + Duration::days(7)).to_rfc3339();
    let due = (now + Duration::days(6)).to_rfc3339();
    let response = client
        .patch(format!("{root}/api/items/{item_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"status":"to_do","plannedSeconds":3600,"devDueAt":due}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let response = client
        .get(format!("{root}/api/schedule"))
        .header("Cookie", &cookie)
        .query(&[("from", &from), ("to", &to), ("kind", &"work".to_owned())])
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let report: Value = response.json().await.unwrap();
    assert!(report["lanes"]
        .as_array()
        .unwrap()
        .iter()
        .flat_map(|lane| lane["blocks"].as_array().unwrap())
        .any(|block| block["itemId"] == item_id));
    let response = client
        .get(format!("{root}/api/schedule"))
        .header("Cookie", &cookie)
        .query(&[("from", &from), ("to", &to)])
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let response = client
        .post(format!("{root}/api/schedule/whatif"))
        .header("Cookie", &cookie)
        .json(&json!({"from":from,"to":to,"dropItems":[item_id]}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let comparison: Value = response.json().await.unwrap();
    assert!(comparison["base"]["score"].is_number());
    assert!(comparison["variant"]["score"].is_number());
    assert!(comparison["delta"]["score"].is_number());
    client
        .delete(format!("{root}/api/items/{item_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    sqlx::query("DELETE FROM items WHERE id=$1")
        .bind(uuid::Uuid::parse_str(item_id).unwrap())
        .execute(&pool)
        .await
        .unwrap();
    match previous {
        Some((taken_at, blocks)) => {
            sqlx::query("UPDATE schedule_snapshots SET taken_at=$1,blocks=$2 WHERE kind='work'")
                .bind(taken_at)
                .bind(blocks)
                .execute(&pool)
                .await
                .unwrap();
        }
        None => {
            sqlx::query("DELETE FROM schedule_snapshots WHERE kind='work'")
                .execute(&pool)
                .await
                .unwrap();
        }
    }
    server.abort();
}
