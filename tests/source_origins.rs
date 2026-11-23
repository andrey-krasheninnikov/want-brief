#[path = "support/workspaces.rs"]
mod workspaces;

use axum::{http::StatusCode, Router};
use reqwest::Method;
use serde_json::json;
use sqlx::PgPool;
use std::sync::{
    atomic::{AtomicUsize, Ordering},
    Arc,
};
use uuid::Uuid;
use want_brief::crypto::TokenBox;
use workspaces::{account, start, value};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn new_accounts_check_source_addresses_on_save_and_every_sync() {
    let url = std::env::var("TEST_DATABASE_URL").unwrap();
    assert!(url
        .split('?')
        .next()
        .unwrap()
        .rsplit('/')
        .next()
        .unwrap()
        .starts_with("wantbrief_issue92_"));
    let control = PgPool::connect(&url).await.unwrap();
    let mut tx = control.begin().await.unwrap();
    let owner = account(&mut tx).await;
    tx.commit().await.unwrap();
    let (api, server) = start(&control).await;
    let project: Uuid = sqlx::query_scalar("SELECT id FROM projects WHERE workspace_id=$1 LIMIT 1")
        .bind(owner.personal)
        .fetch_one(&control)
        .await
        .unwrap();
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = format!("http://{}", listener.local_addr().unwrap());
    let hits = Arc::new(AtomicUsize::new(0));
    let observed = hits.clone();
    let remote = tokio::spawn(async move {
        let app = Router::new().fallback(move || async move {
            observed.fetch_add(1, Ordering::SeqCst);
            StatusCode::OK
        });
        axum::serve(listener, app).await.unwrap();
    });
    let request = |method, path| api.request(method, &owner, owner.personal, path);
    value(
        request(Method::POST, "sources")
            .json(&json!({"name":"External","kind":"todoist","baseUrl":address,"token":"test"})),
        StatusCode::BAD_REQUEST,
    )
    .await;
    let source = value(
        request(Method::POST, "sources")
            .json(&json!({"name":"External","kind":"todoist","projectId":project})),
        StatusCode::CREATED,
    )
    .await;
    let source_id: Uuid = source["id"].as_str().unwrap().parse().unwrap();
    let source_path = format!("sources/{source_id}");
    value(
        request(Method::PATCH, &source_path).json(&json!({"baseUrl":address})),
        StatusCode::BAD_REQUEST,
    )
    .await;

    // A row already on disk is rechecked independently of the settings controller.
    let sealed = TokenBox::new(&"11".repeat(32))
        .unwrap()
        .seal("test")
        .unwrap();
    sqlx::query("UPDATE sources SET base_url=$1,token_sealed=$2 WHERE id=$3")
        .bind(address)
        .bind(sealed)
        .bind(source_id)
        .execute(&control)
        .await
        .unwrap();
    let item:Uuid=sqlx::query_scalar("INSERT INTO items(workspace_id,source_id,external_key,title,kind,status) VALUES($1,$2,'123','Task','task','to_do') RETURNING id")
        .bind(owner.personal).bind(source_id).fetch_one(&control).await.unwrap();
    for path in [format!("{source_path}/sync"), format!("items/{item}/sync")] {
        value(
            api.request(Method::POST, &owner, owner.personal, &path),
            StatusCode::FORBIDDEN,
        )
        .await;
    }
    assert_eq!(hits.load(Ordering::SeqCst), 0);
    server.abort();
    remote.abort();
    let mut tx = control.begin().await.unwrap();
    for table in ["items", "sources", "projects"] {
        sqlx::query(&format!("DELETE FROM {table} WHERE workspace_id=$1"))
            .bind(owner.personal)
            .execute(&mut *tx)
            .await
            .unwrap();
    }
    sqlx::query("DELETE FROM workspaces WHERE id=$1")
        .bind(owner.personal)
        .execute(&mut *tx)
        .await
        .unwrap();
    sqlx::query("DELETE FROM users WHERE id=$1")
        .bind(owner.id)
        .execute(&mut *tx)
        .await
        .unwrap();
    tx.commit().await.unwrap();
}
