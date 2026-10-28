use chrono::{Duration, Utc};
use serde_json::{json, Value};
use sqlx::PgPool;
use std::sync::Arc;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn timer_and_checkin_routes_keep_history_and_defaults() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let source_id = Uuid::new_v4();
    let item_id = Uuid::new_v4();
    sqlx::query("INSERT INTO sources (id,kind,name) VALUES ($1,'manual','Track test')")
        .bind(source_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("INSERT INTO items (id,source_id,title,status,kind) VALUES ($1,$2,'Track test','to_do','task')")
        .bind(item_id)
        .bind(source_id)
        .execute(&pool)
        .await
        .unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
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
    let latest: Value = client
        .get(format!("{root}/api/checkins/latest"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(latest["focus"], 3);
    let response = client
        .post(format!("{root}/api/checkins"))
        .header("Cookie", &cookie)
        .json(&json!({"kind":"focus","level":5}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let checkin: Value = response.json().await.unwrap();
    assert_eq!(checkin["kind"], "focus");
    let response = client
        .post(format!("{root}/api/intervals"))
        .header("Cookie", &cookie)
        .json(&json!({"itemId":item_id.to_string()}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let interval: Value = response.json().await.unwrap();
    assert!(interval["endedAt"].is_null());
    let interval_id = interval["id"].as_str().unwrap();
    let response = client
        .post(format!("{root}/api/intervals/{interval_id}/stop"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let interval: Value = response.json().await.unwrap();
    assert!(interval["endedAt"].is_string());
    let kinds: Vec<String> =
        sqlx::query_scalar("SELECT kind FROM item_events WHERE item_id=$1 ORDER BY created_at")
            .bind(item_id)
            .fetch_all(&pool)
            .await
            .unwrap();
    assert_eq!(kinds, ["timer_start", "timer_stop"]);
    let start = Utc::now() - Duration::hours(1);
    let end = start + Duration::minutes(30);
    let response = client
        .post(format!("{root}/api/intervals"))
        .header("Cookie", &cookie)
        .json(&json!({"itemId":item_id.to_string(),"startedAt":start.to_rfc3339(),"endedAt":end.to_rfc3339()}))
        .send().await.unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let logged: Value = response.json().await.unwrap();
    assert!(logged["endedAt"].is_string());
    let duration: String = sqlx::query_scalar(
        "SELECT to_value FROM item_events WHERE item_id=$1 AND kind='timer_log'",
    )
    .bind(item_id)
    .fetch_one(&pool)
    .await
    .unwrap();
    assert_eq!(duration, "1800");
    sqlx::query("DELETE FROM stress_logs WHERE id=$1")
        .bind(Uuid::parse_str(checkin["id"].as_str().unwrap()).unwrap())
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("DELETE FROM sources WHERE id=$1")
        .bind(source_id)
        .execute(&pool)
        .await
        .unwrap();
    server.abort();
}
