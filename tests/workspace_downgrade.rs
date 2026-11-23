#[path = "support/workspaces.rs"]
mod support;

use reqwest::{Method, StatusCode};
use serde_json::{json, Value};
use sqlx::PgPool;
use support::{account, space, start, value};
use uuid::Uuid;

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn downgraded_editor_keeps_private_records_without_shared_writes() {
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
    let editor = account(&mut tx).await;
    let team = space(&mut tx, owner.id, "team", "paid").await;
    sqlx::query("INSERT INTO workspace_members VALUES($1,$2,'editor')")
        .bind(team)
        .bind(editor.id)
        .execute(&mut *tx)
        .await
        .unwrap();
    tx.commit().await.unwrap();
    let (api, server) = start(&control).await;
    let item = value(
        api.request(Method::POST, &owner, team, "items")
            .json(&json!({"title":"Shared task","kind":"task"})),
        StatusCode::CREATED,
    )
    .await;
    let item_id: Uuid = item["id"].as_str().unwrap().parse().unwrap();
    let path = format!("items/{item_id}");
    let person = value(
        api.request(Method::POST, &owner, team, "people")
            .json(&json!({"name":"Colleague"})),
        StatusCode::CREATED,
    )
    .await;
    let person_path = format!("people/{}", person["id"].as_str().unwrap());
    let company = value(
        api.request(Method::POST, &owner, team, "companies")
            .json(&json!({"name":"Company"})),
        StatusCode::CREATED,
    )
    .await;
    let company_path = format!("companies/{}", company["id"].as_str().unwrap());
    let timer = value(
        api.request(Method::POST, &editor, team, "intervals")
            .json(&json!({"itemId":item_id})),
        StatusCode::CREATED,
    )
    .await;
    let stop = format!("intervals/{}/stop", timer["id"].as_str().unwrap());
    let shared_version: String = sqlx::query_scalar("SELECT xmin::text FROM items WHERE id=$1")
        .bind(item_id)
        .fetch_one(&control)
        .await
        .unwrap();
    sqlx::query("UPDATE workspaces SET plan='free' WHERE id=$1")
        .bind(team)
        .execute(&control)
        .await
        .unwrap();

    value(
        api.request(Method::POST, &owner, team, &stop),
        StatusCode::NOT_FOUND,
    )
    .await;
    value(
        api.request(Method::POST, &editor, team, &stop),
        StatusCode::OK,
    )
    .await;
    value(
        api.request(Method::POST, &editor, team, "notes")
            .json(&json!({"body":"Personal note","itemId":item_id})),
        StatusCode::CREATED,
    )
    .await;
    value(
        api.request(Method::POST, &editor, team, "checkins")
            .json(&json!({"kind":"energy","level":4})),
        StatusCode::CREATED,
    )
    .await;
    value(
        api.request(Method::POST, &editor, team, "stress")
            .json(&json!({"itemId":item_id,"level":4})),
        StatusCode::CREATED,
    )
    .await;
    value(
        api.request(
            Method::POST,
            &editor,
            team,
            &format!("{path}/events/annotate"),
        )
        .json(&json!({"kinds":["timer_stop"],"note":"My timer","stress":2})),
        StatusCode::OK,
    )
    .await;
    value(
        api.request(Method::PUT, &editor, team, "schedule/settings")
            .json(&json!({"timezone":"UTC"})),
        StatusCode::OK,
    )
    .await;
    value(
        api.request(Method::PATCH, &editor, team, &path)
            .json(&json!({"stress":5})),
        StatusCode::OK,
    )
    .await;
    value(
        api.request(Method::PATCH, &editor, team, &path)
            .json(&json!({"stress":1,"title":"Replacement"})),
        StatusCode::FORBIDDEN,
    )
    .await;
    value(
        api.request(Method::DELETE, &editor, team, &path),
        StatusCode::FORBIDDEN,
    )
    .await;
    value(
        api.request(Method::PATCH, &editor, team, &person_path)
            .json(&json!({"bornOn":"1990-01-01"})),
        StatusCode::OK,
    )
    .await;
    value(
        api.request(Method::PATCH, &editor, team, &person_path)
            .json(&json!({"name":"Replacement","bornOn":"1980-01-01"})),
        StatusCode::FORBIDDEN,
    )
    .await;
    value(
        api.request(
            Method::POST,
            &editor,
            team,
            &format!("{company_path}/titles"),
        )
        .json(&json!({"title":"Private role"})),
        StatusCode::OK,
    )
    .await;
    let mine = value(
        api.request(Method::GET, &editor, team, &path),
        StatusCode::OK,
    )
    .await;
    assert_eq!(mine["stress"], 5);
    let other = value(
        api.request(Method::GET, &owner, team, &path),
        StatusCode::OK,
    )
    .await;
    assert_eq!(other["title"], "Shared task");
    assert_eq!(other["stress"], Value::Null);
    assert!(!value(
        api.request(Method::GET, &owner, team, &format!("{path}/events")),
        StatusCode::OK
    )
    .await
    .to_string()
    .contains("My timer"));
    assert!(!value(
        api.request(Method::GET, &owner, team, "notes"),
        StatusCode::OK
    )
    .await
    .to_string()
    .contains("Personal note"));
    assert!(value(
        api.request(Method::GET, &owner, team, &person_path),
        StatusCode::OK
    )
    .await["bornOn"]
        .is_null());
    assert!(!value(
        api.request(Method::GET, &owner, team, &company_path),
        StatusCode::OK
    )
    .await
    .to_string()
    .contains("Private role"));
    let after: String = sqlx::query_scalar("SELECT xmin::text FROM items WHERE id=$1")
        .bind(item_id)
        .fetch_one(&control)
        .await
        .unwrap();
    assert_eq!(after, shared_version);

    server.abort();
    let ids = [team, owner.personal, editor.personal];
    let mut tx = control.begin().await.unwrap();
    for table in [
        "notes",
        "time_intervals",
        "stress_logs",
        "schedule_settings",
        "items",
        "companies",
        "people",
        "sources",
        "projects",
    ] {
        sqlx::query(&format!("DELETE FROM {table} WHERE workspace_id=ANY($1)"))
            .bind(&ids[..])
            .execute(&mut *tx)
            .await
            .unwrap();
    }
    sqlx::query("DELETE FROM workspaces WHERE id=ANY($1)")
        .bind(&ids[..])
        .execute(&mut *tx)
        .await
        .unwrap();
    sqlx::query("DELETE FROM users WHERE id=ANY($1)")
        .bind(&[owner.id, editor.id][..])
        .execute(&mut *tx)
        .await
        .unwrap();
    tx.commit().await.unwrap();
}
