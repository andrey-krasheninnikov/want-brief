use serde_json::{json, Value};
use sqlx::types::Json as DbJson;
use sqlx::PgPool;
use std::sync::Arc;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn schedule_settings_keep_defaults_and_day_overrides_round_trip() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
    let previous: Option<DbJson<Value>> =
        sqlx::query_scalar("SELECT data FROM schedule_settings WHERE id=1")
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
    let response = client
        .put(format!("{root}/api/schedule/settings"))
        .header("Cookie", &cookie)
        .json(&json!({"timezone":"America/New_York"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let settings: Value = response.json().await.unwrap();
    assert_eq!(settings["timezone"], "America/New_York");
    assert_eq!(settings["workStartMin"], 600);
    assert_eq!(settings["checkWindows"], json!(["10:00", "14:00"]));
    let response = client
        .put(format!("{root}/api/schedule/settings"))
        .header("Cookie", &cookie)
        .json(&json!({"workStartMin":1100,"workEndMin":1000}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::BAD_REQUEST);
    let response = client
        .put(format!("{root}/api/schedule/days/2026-11-03"))
        .header("Cookie", &cookie)
        .json(&json!({"off":true,"note":"Holiday"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let day: Value = response.json().await.unwrap();
    assert_eq!(day["off"], true);
    assert_eq!(day["workStartMin"], Value::Null);
    let response = client
        .get(format!("{root}/api/schedule/days"))
        .header("Cookie", &cookie)
        .query(&[("from", "2026-11-01"), ("to", "2026-11-07")])
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let days: Value = response.json().await.unwrap();
    assert!(days
        .as_array()
        .unwrap()
        .iter()
        .any(|value| value["day"] == "2026-11-03"));
    let response = client
        .delete(format!("{root}/api/schedule/days/2026-11-03"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    match previous {
        Some(value) => {
            sqlx::query("UPDATE schedule_settings SET data=$1 WHERE id=1")
                .bind(value)
                .execute(&pool)
                .await
                .unwrap();
        }
        None => {
            sqlx::query("DELETE FROM schedule_settings WHERE id=1")
                .execute(&pool)
                .await
                .unwrap();
        }
    }
    server.abort();
}
