use serde_json::{json, Value};
use sqlx::PgPool;
use std::sync::Arc;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn event_series_routes_preserve_recurrence_and_timezone() {
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
    let response = client.post(format!("{root}/api/events")).header("Cookie", &cookie)
        .json(&json!({"title":"Weekly check","kind":"call","startsAt":"2026-10-26T08:00:00Z","timezone":"Europe/Moscow","durationSeconds":1800,"recurrence":"weekly","weekdays":[0,7],"repeatUntil":"2026-11-30"}))
        .send().await.unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let event: Value = response.json().await.unwrap();
    assert_eq!(event["recurrence"], "weekly");
    assert_eq!(event["timezone"], "Europe/Moscow");
    assert_eq!(event["weekdays"], json!([0, 7]));
    assert_eq!(event["people"], json!([]));
    let event_id = event["id"].as_str().unwrap();
    let response = client
        .get(format!("{root}/api/event-series"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let series: Value = response.json().await.unwrap();
    assert!(series
        .as_array()
        .unwrap()
        .iter()
        .any(|value| value["id"] == event_id));
    let response = client.patch(format!("{root}/api/events/{event_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"title":"Renamed check","kind":"call","startsAt":"2026-10-26T08:00:00Z","durationSeconds":1800,"recurrence":"weekly","weekdays":[0,7]}))
        .send().await.unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let changed: Value = response.json().await.unwrap();
    assert_eq!(changed["title"], "Renamed check");
    assert_eq!(changed["timezone"], "Europe/Moscow");
    let response = client
        .get(format!("{root}/api/events"))
        .header("Cookie", &cookie)
        .query(&[
            ("from", "2026-10-26T00:00:00Z"),
            ("to", "2026-11-10T00:00:00Z"),
        ])
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let occurrences: Value = response.json().await.unwrap();
    let matching: Vec<&Value> = occurrences
        .as_array()
        .unwrap()
        .iter()
        .filter(|value| value["seriesId"] == event_id)
        .collect();
    assert_eq!(matching.len(), 3);
    assert_eq!(matching[2]["originalOn"], "2026-11-09");
    assert_eq!(matching[0]["originalOn"], "2026-10-26");
    assert_eq!(matching[1]["originalOn"], "2026-11-02");
    let response = client
        .put(format!("{root}/api/events/{event_id}/occurrence"))
        .header("Cookie", &cookie)
        .json(&json!({"originalOn":"2026-11-02","startsAt":"2026-11-03T08:00:00Z"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let override_row: Value = response.json().await.unwrap();
    assert_eq!(override_row["overridden"], true);
    assert_eq!(override_row["startsAt"], "2026-11-03T08:00:00Z");
    let response = client
        .get(format!("{root}/api/events"))
        .header("Cookie", &cookie)
        .query(&[
            ("from", "2026-10-26T00:00:00Z"),
            ("to", "2026-11-10T00:00:00Z"),
        ])
        .send()
        .await
        .unwrap();
    let occurrences: Value = response.json().await.unwrap();
    assert!(occurrences
        .as_array()
        .unwrap()
        .iter()
        .any(|value| value["seriesId"] == event_id && value["startsAt"] == "2026-11-03T08:00:00Z"));
    let response = client
        .delete(format!("{root}/api/events/{event_id}/occurrence"))
        .header("Cookie", &cookie)
        .query(&[("on", "2026-11-02")])
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let response = client
        .post(format!("{root}/api/events/{event_id}/notes"))
        .header("Cookie", &cookie)
        .json(&json!({"originalOn":"2026-11-02","body":"Meeting note"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let note: Value = response.json().await.unwrap();
    assert_eq!(note["body"], "Meeting note");
    let note_id = note["id"].as_str().unwrap();
    let response = client
        .get(format!("{root}/api/events/{event_id}/notes"))
        .header("Cookie", &cookie)
        .query(&[("on", "2026-11-02")])
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let notes: Value = response.json().await.unwrap();
    assert!(notes
        .as_array()
        .unwrap()
        .iter()
        .any(|value| value["id"] == note_id));
    let response = client
        .delete(format!("{root}/api/events/{event_id}/notes/{note_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let response = client
        .delete(format!("{root}/api/events/{event_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let response = client.post(format!("{root}/api/events")).header("Cookie", &cookie)
        .json(&json!({"title":"DST check","startsAt":"2026-03-01T07:30:00Z","timezone":"America/New_York","recurrence":"weekly","weekdays":[6,13]}))
        .send().await.unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let dst_event: Value = response.json().await.unwrap();
    let dst_id = dst_event["id"].as_str().unwrap();
    let response = client
        .get(format!("{root}/api/events"))
        .header("Cookie", &cookie)
        .query(&[
            ("from", "2026-03-07T00:00:00Z"),
            ("to", "2026-03-10T00:00:00Z"),
        ])
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let occurrences: Value = response.json().await.unwrap();
    let dst = occurrences
        .as_array()
        .unwrap()
        .iter()
        .find(|value| value["seriesId"] == dst_id)
        .unwrap();
    assert_eq!(dst["startsAt"], "2026-03-08T07:00:00Z");
    let response = client
        .delete(format!("{root}/api/events/{dst_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    server.abort();
}
