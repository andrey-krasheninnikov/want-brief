use chrono::{DateTime, Utc};
use serde_json::Value;
use sqlx::PgPool;
use std::sync::Arc;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn load_counts_overlaps_and_splits_moscow_midnight() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
    let source_id: Uuid = sqlx::query_scalar("SELECT id FROM sources WHERE kind='manual' LIMIT 1")
        .fetch_one(&pool)
        .await
        .unwrap();
    let project_id: Uuid = sqlx::query_scalar("SELECT id FROM projects WHERE name='Work' LIMIT 1")
        .fetch_one(&pool)
        .await
        .unwrap();
    let first = Uuid::new_v4();
    let second = Uuid::new_v4();
    for item_id in [first, second] {
        sqlx::query("INSERT INTO items (id,source_id,title,status,kind,project_id) VALUES ($1,$2,'Load test','in_progress','task',$3)")
            .bind(item_id).bind(source_id).bind(project_id).execute(&pool).await.unwrap();
    }
    for (item_id, start, end) in [
        (first, "2026-01-01T20:30:00Z", "2026-01-01T21:30:00Z"),
        (second, "2026-01-01T21:00:00Z", "2026-01-01T22:00:00Z"),
    ] {
        sqlx::query(
            "INSERT INTO time_intervals (id,item_id,started_at,ended_at) VALUES ($1,$2,$3,$4)",
        )
        .bind(Uuid::new_v4())
        .bind(item_id)
        .bind(
            DateTime::parse_from_rfc3339(start)
                .unwrap()
                .with_timezone(&Utc),
        )
        .bind(
            DateTime::parse_from_rfc3339(end)
                .unwrap()
                .with_timezone(&Utc),
        )
        .execute(&pool)
        .await
        .unwrap();
    }
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
    let response = reqwest::Client::new()
        .get(format!("{root}/api/load"))
        .header("Cookie", format!("wb_session={token}"))
        .query(&[
            ("from", "2026-01-01T20:00:00Z"),
            ("to", "2026-01-01T22:00:00Z"),
        ])
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let report: Value = response.json().await.unwrap();
    assert_eq!(report["allocatedSeconds"], 7200);
    assert_eq!(report["wallSeconds"], 5400);
    assert_eq!(report["byDay"][0]["date"], "2026-01-01");
    assert_eq!(report["byDay"][1]["date"], "2026-01-02");
    assert_eq!(report["byDay"][0]["allocatedSeconds"], 1800);
    assert_eq!(report["byDay"][1]["allocatedSeconds"], 5400);
    sqlx::query("DELETE FROM items WHERE id=ANY($1)")
        .bind(vec![first, second])
        .execute(&pool)
        .await
        .unwrap();
    server.abort();
}
