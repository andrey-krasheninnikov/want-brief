use serde_json::{json, Value};
use sqlx::PgPool;
use std::sync::Arc;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn item_routes_preserve_shape_and_soft_delete() {
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
    let response = client
        .post(format!("{root}/api/items"))
        .header("Cookie", &cookie)
        .json(&json!({"title":"  Item test  ","kind":"request","urgent":true}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let item: Value = response.json().await.unwrap();
    assert_eq!(item["title"], "Item test");
    assert_eq!(item["status"], "backlog");
    assert_eq!(item["occupancy"], "waiting");
    assert_eq!(item["quadrant"], "delegate");
    assert_eq!(item["personIds"], json!([]));
    assert_eq!(item["links"], json!([]));
    let item_id = item["id"].as_str().unwrap();
    let response = client
        .patch(format!("{root}/api/items/{item_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"status":"review"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::BAD_REQUEST);
    let response = client
        .patch(format!("{root}/api/items/{item_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"title":" Updated ","status":"in_progress","stress":4,"plannedSeconds":3600}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let patched: Value = response.json().await.unwrap();
    assert_eq!(patched["title"], "Updated");
    assert_eq!(patched["status"], "in_progress");
    assert_eq!(patched["stress"], 4);
    let event_count: i64 = sqlx::query_scalar("SELECT count(*) FROM item_events WHERE item_id=$1")
        .bind(Uuid::parse_str(item_id).unwrap())
        .fetch_one(&pool)
        .await
        .unwrap();
    assert_eq!(event_count, 4);
    let response = client
        .post(format!("{root}/api/items/{item_id}/notes"))
        .header("Cookie", &cookie)
        .json(&json!({"body":"  Item note  "}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let note: Value = response.json().await.unwrap();
    assert_eq!(note["body"], "Item note");
    let note_id = note["id"].as_str().unwrap();
    let response = client
        .get(format!("{root}/api/items/{item_id}/notes"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert_eq!(response.json::<Value>().await.unwrap()[0]["id"], note_id);
    let response = client
        .delete(format!("{root}/api/items/{item_id}/notes/{note_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let response = client
        .post(format!("{root}/api/items/{item_id}/checks"))
        .header("Cookie", &cookie)
        .json(&json!({"body":"First"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let first: Value = response.json().await.unwrap();
    let first_id = first["id"].as_str().unwrap();
    let response = client
        .post(format!("{root}/api/items/{item_id}/checks"))
        .header("Cookie", &cookie)
        .json(&json!({"body":"Second"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let second: Value = response.json().await.unwrap();
    let second_id = second["id"].as_str().unwrap();
    let response = client
        .patch(format!("{root}/api/items/{item_id}/checks/{first_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"done":true}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert_eq!(response.json::<Value>().await.unwrap()["done"], true);
    let response = client
        .put(format!("{root}/api/items/{item_id}/checks/order"))
        .header("Cookie", &cookie)
        .json(&json!({"ids":[second_id,first_id]}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let checks: Value = response.json().await.unwrap();
    assert_eq!(checks[0]["id"], second_id);
    let response = client
        .delete(format!("{root}/api/items/{item_id}/checks/{first_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let response = client
        .get(format!("{root}/api/items/{item_id}/events"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert!(
        response
            .json::<Value>()
            .await
            .unwrap()
            .as_array()
            .unwrap()
            .len()
            >= 4
    );
    let response = client
        .post(format!("{root}/api/items/{item_id}/events/annotate"))
        .header("Cookie", &cookie)
        .json(&json!({"kinds":["status"],"note":"  Status context  ","stress":2}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let event: Value = response.json().await.unwrap();
    assert_eq!(event["note"], "Status context");
    assert_eq!(event["stress"], 2);
    let response = client
        .get(format!("{root}/api/items?kind=request&openOnly=true"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let items: Value = response.json().await.unwrap();
    assert!(items
        .as_array()
        .unwrap()
        .iter()
        .any(|value| value["id"] == item_id));
    let response = client
        .delete(format!("{root}/api/items/{item_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let response = client
        .get(format!("{root}/api/items/{item_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NOT_FOUND);
    let response = client
        .post(format!("{root}/api/items/{item_id}/undelete"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert!(response.json::<Value>().await.unwrap()["deletedAt"].is_null());
    sqlx::query("DELETE FROM stress_logs WHERE item_id=$1")
        .bind(Uuid::parse_str(item_id).unwrap())
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("DELETE FROM items WHERE id=$1")
        .bind(Uuid::parse_str(item_id).unwrap())
        .execute(&pool)
        .await
        .unwrap();
    server.abort();
}
