#[path = "support/workspaces.rs"]
mod workspaces;

use reqwest::{Method, StatusCode};
use serde_json::{json, Value};
use sqlx::PgPool;
use std::sync::Arc;
use std::time::Duration;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn project_routes_keep_response_shape() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
    let app = router(AppState {
        pool,
        auth,
        cors_origin: "http://127.0.0.1:5173".to_owned(),
        token_box: Arc::new(
            TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
                .unwrap(),
        ),
    });
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = listener.local_addr().unwrap();
    let server = tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    let client = reqwest::Client::new();
    let root = format!("http://{address}");
    let response = client
        .post(format!("{root}/api/projects"))
        .header("Cookie", format!("wb_session={token}"))
        .json(&json!({"name":"  Project parity  ","targetHoursDay":2.0}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let created: Value = response.json().await.unwrap();
    assert_eq!(created["name"], "Project parity");
    assert_eq!(created["targetHoursWeek"], 14.0);
    assert_eq!(created["people"], json!([]));
    assert_eq!(created["companies"], json!([]));
    let id = created["id"].as_str().unwrap();
    let response = client
        .get(format!("{root}/api/projects/{id}"))
        .header("Cookie", format!("wb_session={token}"))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert_eq!(response.json::<Value>().await.unwrap()["id"], id);
    let response = client
        .patch(format!("{root}/api/projects/{id}"))
        .header("Cookie", format!("wb_session={token}"))
        .json(&json!({"archived":true}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert!(response.json::<Value>().await.unwrap()["archivedAt"].is_string());
    let response = client
        .post(format!("{root}/api/projects/{id}/notes"))
        .header("Cookie", format!("wb_session={token}"))
        .json(&json!({"body":"  project note  "}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let note: Value = response.json().await.unwrap();
    assert_eq!(note["body"], "project note");
    let response = client
        .delete(format!(
            "{root}/api/projects/{id}/notes/{}",
            note["id"].as_str().unwrap()
        ))
        .header("Cookie", format!("wb_session={token}"))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let response = client
        .delete(format!("{root}/api/projects/{id}"))
        .header("Cookie", format!("wb_session={token}"))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    let response = client
        .post(format!("{root}/api/notes"))
        .header("Cookie", format!("wb_session={token}"))
        .json(&json!({"body":"  personal note  "}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let note: Value = response.json().await.unwrap();
    assert_eq!(note["body"], "personal note");
    assert!(note["itemId"].is_null());
    let note_id = note["id"].as_str().unwrap();
    let response = client
        .patch(format!("{root}/api/notes/{note_id}"))
        .header("Cookie", format!("wb_session={token}"))
        .json(&json!({"body":"changed"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert_eq!(response.json::<Value>().await.unwrap()["body"], "changed");
    let response = client
        .delete(format!("{root}/api/notes/{note_id}"))
        .header("Cookie", format!("wb_session={token}"))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    server.abort();
}

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn concurrent_project_patches_keep_each_members_changes() {
    use workspaces::{account, space, start, value};

    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let mut setup = pool.begin().await.unwrap();
    let owner = account(&mut setup).await;
    let editor = account(&mut setup).await;
    let team = space(&mut setup, owner.id, "team", "paid").await;
    sqlx::query("INSERT INTO workspace_members(workspace_id,user_id,role) VALUES($1,$2,'editor')")
        .bind(team)
        .bind(editor.id)
        .execute(&mut *setup)
        .await
        .unwrap();
    let project: Uuid = sqlx::query_scalar("SELECT id FROM projects WHERE workspace_id=$1 LIMIT 1")
        .bind(team)
        .fetch_one(&mut *setup)
        .await
        .unwrap();
    let person: Uuid = sqlx::query_scalar(
        "INSERT INTO people(workspace_id,name) VALUES($1,'Teammate') RETURNING id",
    )
    .bind(team)
    .fetch_one(&mut *setup)
    .await
    .unwrap();
    setup.commit().await.unwrap();
    let (api, server) = start(&pool).await;
    let path = format!("projects/{project}");
    let person_link = json!({"id":person,"comment":"Contributor"});
    value(
        api.request(Method::PATCH, &editor, team, &path)
            .json(&json!({"monthlyIncomeUsd":12,"monthlyIncomeRub":30})),
        StatusCode::OK,
    )
    .await;

    // Hold the row until both HTTP requests are waiting for their write.
    let mut lock = pool.begin().await.unwrap();
    let blocker: i32 =
        sqlx::query_scalar("SELECT pg_backend_pid() FROM projects WHERE id=$1 FOR UPDATE")
            .bind(project)
            .fetch_one(&mut *lock)
            .await
            .unwrap();
    let rename = tokio::spawn(
        api.request(Method::PATCH, &owner, team, &path)
            .json(&json!({"name":"Renamed project","people":[person_link]}))
            .send(),
    );
    let recolor = tokio::spawn(
        api.request(Method::PATCH, &editor, team, &path)
            .json(&json!({"color":"#778899","companies":[]}))
            .send(),
    );
    tokio::time::timeout(Duration::from_secs(10), async {
        loop {
            let writers: i64 = sqlx::query_scalar(
                "WITH RECURSIVE waiting(pid) AS (
                    SELECT $1::int UNION SELECT a.pid FROM pg_stat_activity a
                    JOIN waiting w ON w.pid=ANY(pg_blocking_pids(a.pid))
                 ) SELECT count(*)-1 FROM waiting",
            )
            .bind(blocker)
            .fetch_one(&pool)
            .await
            .unwrap();
            if writers == 2 {
                break;
            }
            tokio::time::sleep(Duration::from_millis(10)).await;
        }
    })
    .await
    .expect("both project patches should reach the locked row");
    lock.commit().await.unwrap();
    for request in [rename, recolor] {
        let response = request.await.unwrap().unwrap();
        assert_eq!(
            response.status(),
            StatusCode::OK,
            "{}",
            response.text().await.unwrap()
        );
    }
    let shared = value(
        api.request(Method::GET, &owner, team, &path),
        StatusCode::OK,
    )
    .await;
    assert_eq!(shared["name"], "Renamed project");
    assert_eq!(shared["color"], "#778899");
    assert_eq!(shared["people"], json!([person_link]));
    assert_eq!(shared["monthlyIncomeUsd"], 0.0);

    sqlx::query("UPDATE workspaces SET plan='free' WHERE id=$1")
        .bind(team)
        .execute(&pool)
        .await
        .unwrap();
    value(
        api.request(Method::PATCH, &editor, team, &path)
            .json(&json!({"name":"Unexpected rename","monthlyIncomeUsd":100})),
        StatusCode::FORBIDDEN,
    )
    .await;
    let private = value(
        api.request(Method::PATCH, &editor, team, &path)
            .json(&json!({"monthlyIncomeRub":40})),
        StatusCode::OK,
    )
    .await;
    assert_eq!(private["name"], "Renamed project");
    assert_eq!(private["monthlyIncomeUsd"], 12.0);
    assert_eq!(private["monthlyIncomeRub"], 40.0);
    let foreign = api
        .request(Method::PATCH, &editor, editor.personal, &path)
        .json(&json!({"monthlyIncomeUsd":99}))
        .send()
        .await
        .unwrap();
    assert_eq!(foreign.status(), StatusCode::NOT_FOUND);
    server.abort();

    let mut cleanup = pool.begin().await.unwrap();
    let spaces = [team, owner.personal, editor.personal];
    for table in ["people", "sources", "projects"] {
        sqlx::query(&format!("DELETE FROM {table} WHERE workspace_id=ANY($1)"))
            .bind(&spaces[..])
            .execute(&mut *cleanup)
            .await
            .unwrap();
    }
    sqlx::query("DELETE FROM workspaces WHERE id=ANY($1)")
        .bind(&spaces[..])
        .execute(&mut *cleanup)
        .await
        .unwrap();
    sqlx::query("DELETE FROM users WHERE id=ANY($1)")
        .bind(&[owner.id, editor.id][..])
        .execute(&mut *cleanup)
        .await
        .unwrap();
    cleanup.commit().await.unwrap();
}
