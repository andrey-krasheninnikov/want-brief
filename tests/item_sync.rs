mod support;
use axum::routing::get;
use serde_json::{json, Value};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use tokio::sync::Notify;
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
    let pool = support::legacy_pool(&url).await;
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
    let token_box = Arc::new(
        TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff").unwrap(),
    );
    let delayed = Arc::new(AtomicBool::new(false));
    let arrived = Arc::new(Notify::new());
    let release = Arc::new(Notify::new());
    let pause = delayed.clone();
    let ready = arrived.clone();
    let resume = release.clone();
    let late_note = delayed.clone();
    let mock = axum::Router::new()
        .route("/rest/v2/tasks/remote-1", get(move || {
            let pause=pause.clone();
            let ready=ready.clone();
            let resume=resume.clone();
            async move {
                if pause.load(Ordering::SeqCst) {
                    ready.notify_one();
                    resume.notified().await;
                }
                axum::Json(json!({"id":"remote-1","content":"Updated title","description":"Updated description","is_completed":true}))
            }
        }))
        .route("/rest/v2/comments", get(move || {
            let late=late_note.load(Ordering::SeqCst);
            async move {
                let (id,content)=if late {("late","Delayed comment")} else {("comment-1","Remote comment")};
                axum::Json(json!([{"id":id,"content":content,"posted_at":"2026-01-01T12:00:00Z"}]))
            }
        }));
    let mock_listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let mock_url = format!("http://{}", mock_listener.local_addr().unwrap());
    let mock_server = tokio::spawn(async move { axum::serve(mock_listener, mock).await.unwrap() });
    let source_id = Uuid::new_v4();
    let item_id = Uuid::new_v4();
    let project_id: Uuid =
        sqlx::query_scalar("INSERT INTO projects (name) VALUES ('Item sync test') RETURNING id")
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
    delayed.store(true, Ordering::SeqCst);
    sqlx::query("UPDATE items SET title='Keep this version' WHERE id=$1")
        .bind(item_id)
        .execute(&pool)
        .await
        .unwrap();
    for bulk in [false, true] {
        let path = if bulk {
            "/api/items/sync-active".to_owned()
        } else {
            format!("/api/items/{item_id}/sync")
        };
        let request = client
            .post(format!("{root}{path}"))
            .header("Cookie", &cookie);
        let pending = tokio::spawn(async move { request.send().await.unwrap() });
        tokio::time::timeout(std::time::Duration::from_secs(5), arrived.notified())
            .await
            .unwrap();
        if bulk {
            // Reinsert the same ID, source, external key and timestamps as a restore does.
            let mut tx = pool.begin().await.unwrap();
            let row: Value = sqlx::query_scalar("SELECT to_jsonb(i) FROM items i WHERE id=$1")
                .bind(item_id)
                .fetch_one(&mut *tx)
                .await
                .unwrap();
            sqlx::query("DELETE FROM items WHERE id=$1")
                .bind(item_id)
                .execute(&mut *tx)
                .await
                .unwrap();
            sqlx::query("INSERT INTO items SELECT * FROM jsonb_populate_record(NULL::items,$1)")
                .bind(row)
                .execute(&mut *tx)
                .await
                .unwrap();
            tx.commit().await.unwrap();
        } else {
            sqlx::query("UPDATE sources SET query_filter='changed' WHERE id=$1")
                .bind(source_id)
                .execute(&pool)
                .await
                .unwrap();
        }
        release.notify_one();
        let response = pending.await.unwrap();
        if bulk {
            assert_eq!(response.status(), reqwest::StatusCode::OK);
            assert!(
                response.json::<Value>().await.unwrap()["failed"]
                    .as_i64()
                    .unwrap()
                    >= 1
            );
        } else {
            assert_eq!(response.status(), reqwest::StatusCode::CONFLICT);
        }
        let fields:(String,i64)=sqlx::query_as("SELECT title,(SELECT count(*) FROM item_notes WHERE item_id=$1 AND external_id='late') FROM items WHERE id=$1")
            .bind(item_id).fetch_one(&pool).await.unwrap();
        assert_eq!(fields, ("Keep this version".to_owned(), 0));
    }
    sqlx::query("DELETE FROM sources WHERE id=$1")
        .bind(source_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("DELETE FROM projects WHERE id=$1")
        .bind(project_id)
        .execute(&pool)
        .await
        .unwrap();
    server.abort();
    mock_server.abort();
}
