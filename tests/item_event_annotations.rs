#[path = "support/workspaces.rs"]
mod support;

use reqwest::{Method, StatusCode};
use serde_json::{json, Value};
use sqlx::PgPool;
use support::{account, space, start, value, Account, Api};
use uuid::Uuid;

async fn annotate(api: &Api, actor: &Account, space: Uuid, path: &str, body: Value) -> Value {
    value(
        api.request(Method::POST, actor, space, &format!("{path}/annotate"))
            .json(&body),
        StatusCode::OK,
    )
    .await
}

fn event<'a>(history: &'a Value, id: &Value) -> &'a Value {
    history
        .as_array()
        .unwrap()
        .iter()
        .find(|entry| &entry["id"] == id)
        .unwrap()
}

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn shared_history_keeps_its_author_and_private_annotations_through_downgrade_and_restore() {
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
    sqlx::query("INSERT INTO workspace_members(workspace_id,user_id,role) VALUES($1,$2,'editor')")
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
    let item_path = format!("items/{}", item["id"].as_str().unwrap());
    let history_path = format!("{item_path}/events");
    for (actor, status) in [(&editor, "to_do"), (&owner, "in_progress")] {
        value(
            api.request(Method::PATCH, actor, team, &item_path)
                .json(&json!({"status":status})),
            StatusCode::OK,
        )
        .await;
    }
    let baseline = value(
        api.request(Method::GET, &owner, team, &history_path),
        StatusCode::OK,
    )
    .await;
    let editor_event = baseline
        .as_array()
        .unwrap()
        .iter()
        .find(|entry| entry["kind"] == "status" && entry["to"] == "to_do")
        .unwrap();
    let editor_id = &editor_event["id"];
    let owner_event = baseline
        .as_array()
        .unwrap()
        .iter()
        .find(|entry| entry["kind"] == "status" && entry["to"] == "in_progress")
        .unwrap();
    assert_eq!(editor_event["from"], "backlog");
    assert_eq!(editor_event["note"], "");
    assert!(editor_event["stress"].is_null());

    let paid = annotate(
        &api,
        &editor,
        team,
        &history_path,
        json!({"kinds":["status"],"note":"First impression"}),
    )
    .await;
    assert_eq!(paid["id"], *editor_id);
    assert_eq!(paid["note"], "First impression");
    let public = value(
        api.request(Method::GET, &owner, team, &history_path),
        StatusCode::OK,
    )
    .await;
    assert_eq!(public, baseline);

    sqlx::query("UPDATE workspaces SET plan='free' WHERE id=$1")
        .bind(team)
        .execute(&control)
        .await
        .unwrap();
    let mine = annotate(
        &api,
        &editor,
        team,
        &history_path,
        json!({"kinds":["status"],"stress":3}),
    )
    .await;
    assert_eq!(mine["id"], *editor_id);
    assert_eq!(mine["note"], "First impression");
    assert_eq!(mine["stress"], 3);
    let history = value(
        api.request(Method::GET, &editor, team, &history_path),
        StatusCode::OK,
    )
    .await;
    assert_eq!(event(&history, editor_id), &mine);
    assert_eq!(event(&history, &owner_event["id"]), owner_event);
    let other = annotate(
        &api,
        &owner,
        team,
        &history_path,
        json!({"kinds":["status"],"note":"My own change"}),
    )
    .await;
    assert_eq!(other["id"], owner_event["id"]);
    let theirs = value(
        api.request(Method::GET, &owner, team, &history_path),
        StatusCode::OK,
    )
    .await;
    assert_eq!(event(&theirs, editor_id), editor_event);

    let mut archive = value(
        api.request(Method::GET, &owner, team, "backup"),
        StatusCode::OK,
    )
    .await;
    assert_eq!(archive["tables"]["item_event_annotations"], json!([]));
    let public_editor = event(&archive["tables"]["item_events"], editor_id);
    assert_eq!(public_editor["kind"], "status");
    assert_eq!(public_editor["note"], "");
    assert!(public_editor["stress"].is_null());
    assert!(!archive.to_string().contains("First impression"));
    assert!(!archive.to_string().contains("My own change"));
    let restore = "restore?mode=replace";
    let response = api
        .request(Method::POST, &owner, team, restore)
        .json(&archive)
        .send()
        .await
        .unwrap();
    assert_eq!(
        response.status(),
        StatusCode::NO_CONTENT,
        "{}",
        response.text().await.unwrap()
    );
    let author: Uuid = sqlx::query_scalar("SELECT owner_id FROM item_events WHERE id=$1")
        .bind(editor_id.as_str().unwrap().parse::<Uuid>().unwrap())
        .fetch_one(&control)
        .await
        .unwrap();
    assert_eq!(author, editor.id);
    let updated = annotate(
        &api,
        &editor,
        team,
        &history_path,
        json!({"kinds":["status"],"note":"After restore"}),
    )
    .await;
    assert_eq!(updated["id"], *editor_id);
    assert_eq!(updated["stress"], 3);

    archive["tables"]["item_events"]
        .as_array_mut()
        .unwrap()
        .retain(|row| &row["id"] != editor_id);
    value(
        api.request(Method::POST, &owner, team, restore)
            .json(&archive),
        StatusCode::CONFLICT,
    )
    .await;
    let kept = value(
        api.request(Method::GET, &editor, team, &history_path),
        StatusCode::OK,
    )
    .await;
    assert_eq!(event(&kept, editor_id), &updated);
    assert_eq!(event(&kept, &owner_event["id"]), owner_event);

    let personal_item = value(
        api.request(Method::POST, &editor, editor.personal, "items")
            .json(&json!({"title":"Personal task","kind":"task"})),
        StatusCode::CREATED,
    )
    .await;
    let personal_path = format!("items/{}", personal_item["id"].as_str().unwrap());
    value(
        api.request(Method::PATCH, &editor, editor.personal, &personal_path)
            .json(&json!({"status":"in_progress"})),
        StatusCode::OK,
    )
    .await;
    let personal = annotate(
        &api,
        &editor,
        editor.personal,
        &format!("{personal_path}/events"),
        json!({"kinds":["status"],"note":"Personal history","stress":4}),
    )
    .await;
    let personal_backup = value(
        api.request(Method::GET, &editor, editor.personal, "backup"),
        StatusCode::OK,
    )
    .await;
    let stored = event(&personal_backup["tables"]["item_events"], &personal["id"]);
    assert_eq!(stored["note"], "Personal history");
    assert_eq!(stored["stress"], 4);
    assert_eq!(
        personal_backup["tables"]["item_notes"][0]["body"],
        "Personal history"
    );
    assert_eq!(
        personal_backup["tables"]["item_event_annotations"],
        json!([])
    );

    server.abort();
    let spaces = [team, owner.personal, editor.personal];
    let mut tx = control.begin().await.unwrap();
    for table in ["stress_logs", "items", "sources", "projects"] {
        sqlx::query(&format!("DELETE FROM {table} WHERE workspace_id=ANY($1)"))
            .bind(&spaces[..])
            .execute(&mut *tx)
            .await
            .unwrap();
    }
    sqlx::query("DELETE FROM workspaces WHERE id=ANY($1)")
        .bind(&spaces[..])
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
