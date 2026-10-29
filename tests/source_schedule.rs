use axum::extract::State;
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::{routing::get, Json, Router};
use serde_json::{json, Value};
use sqlx::PgPool;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::Notify;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};
use want_brief::sources::run_scheduler;

#[derive(Clone, Default)]
struct Remote {
    fail: Arc<AtomicBool>,
    pause: Arc<AtomicBool>,
    changed_title: Arc<AtomicBool>,
    calls: Arc<AtomicUsize>,
    entered: Arc<Notify>,
    release: Arc<Notify>,
}

async fn tasks(State(remote): State<Remote>) -> Response {
    remote.calls.fetch_add(1, Ordering::SeqCst);
    if remote.pause.load(Ordering::SeqCst) {
        remote.entered.notify_one();
        remote.release.notified().await;
    }
    if remote.fail.load(Ordering::SeqCst) {
        return StatusCode::BAD_GATEWAY.into_response();
    }
    let title = if remote.changed_title.load(Ordering::SeqCst) {
        "Changed task"
    } else {
        "Original task"
    };
    Json(json!([{"id":"scheduled-task","content":title,"priority":3}])).into_response()
}

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn scheduled_sync_records_attempts_preserves_items_and_rejects_overlap() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_issue3_issue4_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
    let state = AppState {
        pool: pool.clone(),
        auth,
        cors_origin: String::new(),
        token_box: Arc::new(
            TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
                .unwrap(),
        ),
    };
    let app_listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let root = format!("http://{}", app_listener.local_addr().unwrap());
    let app = tokio::spawn(async move {
        axum::serve(app_listener, router(state.clone()))
            .await
            .unwrap()
    });
    let remote = Remote::default();
    let mock = Router::new()
        .route("/rest/v2/projects", get(|| async { Json(json!([])) }))
        .route("/rest/v2/tasks", get(tasks))
        .with_state(remote.clone());
    let mock_listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let mock_root = format!("http://{}", mock_listener.local_addr().unwrap());
    let mock_server = tokio::spawn(async move { axum::serve(mock_listener, mock).await.unwrap() });
    let client = reqwest::Client::new();
    let cookie = format!("wb_session={token}");
    let project: Value = client
        .post(format!("{root}/api/projects"))
        .header("Cookie", &cookie)
        .json(&json!({"name":"Schedule test"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    let project_id = project["id"].as_str().unwrap();
    let response = client
        .post(format!("{root}/api/sources"))
        .header("Cookie", &cookie)
        .json(
            &json!({"kind":"todoist","name":"Schedule test","projectId":project_id,
            "baseUrl":mock_root,"token":"test-token","syncIntervalMinutes":15}),
        )
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), StatusCode::CREATED);
    let source: Value = response.json().await.unwrap();
    assert_eq!(source["syncIntervalMinutes"], 15);
    let source_id = source["id"].as_str().unwrap();
    let source_uuid = Uuid::parse_str(source_id).unwrap();
    let response = client
        .patch(format!("{root}/api/sources/{source_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"syncIntervalMinutes":20}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), StatusCode::BAD_REQUEST);

    let scheduler_state = AppState {
        pool: pool.clone(),
        auth: Auth::new(pool.clone()),
        cors_origin: String::new(),
        token_box: Arc::new(
            TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
                .unwrap(),
        ),
    };
    let scheduler = tokio::spawn(run_scheduler(scheduler_state.clone()));
    tokio::time::timeout(Duration::from_secs(5), async {
        loop {
            let done: bool =
                sqlx::query_scalar("SELECT last_sync_at IS NOT NULL FROM sources WHERE id=$1")
                    .bind(source_uuid)
                    .fetch_one(&pool)
                    .await
                    .unwrap();
            if done {
                break;
            }
            tokio::time::sleep(Duration::from_millis(20)).await;
        }
    })
    .await
    .unwrap();
    scheduler.abort();
    let first: Value = client
        .get(format!("{root}/api/sources"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    let first = &first
        .as_array()
        .unwrap()
        .iter()
        .find(|row| row["id"] == source_id)
        .unwrap();
    assert!(first["lastAttemptAt"].is_string());
    assert!(first["lastSyncAt"].is_string());
    let last_success = first["lastSyncAt"].clone();
    assert_eq!(remote.calls.load(Ordering::SeqCst), 1);

    remote.fail.store(true, Ordering::SeqCst);
    sqlx::query(
        "UPDATE sources SET last_attempt_at=clock_timestamp()-INTERVAL '16 minutes' WHERE id=$1",
    )
    .bind(source_uuid)
    .execute(&pool)
    .await
    .unwrap();
    let scheduler = tokio::spawn(run_scheduler(scheduler_state));
    tokio::time::timeout(Duration::from_secs(5), async {
        loop {
            let last_error: String =
                sqlx::query_scalar("SELECT last_error FROM sources WHERE id=$1")
                    .bind(source_uuid)
                    .fetch_one(&pool)
                    .await
                    .unwrap();
            if last_error.contains("HTTP 502") {
                break;
            }
            tokio::time::sleep(Duration::from_millis(20)).await;
        }
    })
    .await
    .unwrap();
    scheduler.abort();
    let after: Value = client
        .get(format!("{root}/api/sources"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    let after = &after
        .as_array()
        .unwrap()
        .iter()
        .find(|row| row["id"] == source_id)
        .unwrap();
    assert_eq!(after["lastSyncAt"], last_success);
    assert!(after["lastAttemptAt"].is_string());
    assert!(after["lastError"].as_str().unwrap().contains("HTTP 502"));
    let title: String = sqlx::query_scalar(
        "SELECT title FROM items WHERE source_id=$1 AND external_key='scheduled-task'",
    )
    .bind(source_uuid)
    .fetch_one(&pool)
    .await
    .unwrap();
    assert_eq!(title, "Original task");

    remote.fail.store(false, Ordering::SeqCst);
    remote.pause.store(true, Ordering::SeqCst);
    remote.changed_title.store(true, Ordering::SeqCst);
    let first_client = client.clone();
    let first_url = format!("{root}/api/sources/{source_id}/sync");
    let first_cookie = cookie.clone();
    let entered = remote.entered.notified();
    let pending = tokio::spawn(async move {
        first_client
            .post(first_url)
            .header("Cookie", first_cookie)
            .send()
            .await
            .unwrap()
    });
    tokio::time::timeout(Duration::from_secs(5), entered)
        .await
        .unwrap();
    let response = client
        .post(format!("{root}/api/sources/{source_id}/sync"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), StatusCode::CONFLICT);
    let response = client
        .patch(format!("{root}/api/sources/{source_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"syncIntervalMinutes":60}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), StatusCode::OK);
    remote.release.notify_one();
    assert_eq!(pending.await.unwrap().status(), StatusCode::CONFLICT);
    let title: String = sqlx::query_scalar(
        "SELECT title FROM items WHERE source_id=$1 AND external_key='scheduled-task'",
    )
    .bind(source_uuid)
    .fetch_one(&pool)
    .await
    .unwrap();
    assert_eq!(title, "Original task");

    remote.pause.store(false, Ordering::SeqCst);
    sqlx::query("UPDATE sources SET sync_claim_id=$2,sync_claim_until=clock_timestamp()-INTERVAL '1 second' WHERE id=$1")
        .bind(source_uuid).bind(Uuid::new_v4()).execute(&pool).await.unwrap();
    let response = client
        .post(format!("{root}/api/sources/{source_id}/sync"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), StatusCode::OK);
    let title: String = sqlx::query_scalar(
        "SELECT title FROM items WHERE source_id=$1 AND external_key='scheduled-task'",
    )
    .bind(source_uuid)
    .fetch_one(&pool)
    .await
    .unwrap();
    assert_eq!(title, "Changed task");

    assert_eq!(
        client
            .delete(format!("{root}/api/sources/{source_id}"))
            .header("Cookie", &cookie)
            .send()
            .await
            .unwrap()
            .status(),
        StatusCode::NO_CONTENT
    );
    assert_eq!(
        client
            .delete(format!("{root}/api/projects/{project_id}"))
            .header("Cookie", &cookie)
            .send()
            .await
            .unwrap()
            .status(),
        StatusCode::NO_CONTENT
    );
    app.abort();
    mock_server.abort();
}
