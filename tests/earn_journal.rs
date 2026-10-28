use chrono::NaiveDate;
use serde_json::{json, Value};
use sqlx::PgPool;
use std::sync::Arc;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn earn_days_and_journal_keep_response_shape() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
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
    let day = "2026-10-29";
    let response = client
        .put(format!("{root}/api/earn/days/{day}"))
        .header("Cookie", &cookie)
        .json(&json!({"usd":10.5,"rub":900.0}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let earn: Value = response.json().await.unwrap();
    assert_eq!(earn["day"], day);
    assert_eq!(earn["rate"], 85.0);
    let response = client
        .get(format!("{root}/api/earn/days?from={day}&to={day}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let days: Value = response.json().await.unwrap();
    assert_eq!(days[0]["usd"], 10.5);
    let response = client
        .post(format!("{root}/api/notes"))
        .header("Cookie", &cookie)
        .json(&json!({"body":"Journal test"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let note: Value = response.json().await.unwrap();
    let response = client
        .get(format!("{root}/api/journal"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let entries: Value = response.json().await.unwrap();
    let entry = entries
        .as_array()
        .unwrap()
        .iter()
        .find(|entry| entry["id"] == note["id"])
        .unwrap();
    assert_eq!(entry["source"], "loose");
    assert_eq!(entry["ownerName"], "Loose");
    sqlx::query("DELETE FROM notes WHERE id=$1::uuid")
        .bind(note["id"].as_str().unwrap())
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("DELETE FROM earn_days WHERE day=$1")
        .bind(NaiveDate::parse_from_str(day, "%Y-%m-%d").unwrap())
        .execute(&pool)
        .await
        .unwrap();
    server.abort();
}
