#[path = "support/workspaces.rs"]
mod workspaces;

use reqwest::{Method, StatusCode};
use serde_json::json;
use sqlx::PgPool;
use workspaces::{account, space, start, value};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn workspace_rights_apply_to_relations_private_fields_restore_and_source_quota() {
    let url = std::env::var("TEST_DATABASE_URL").unwrap();
    assert!(url
        .split('?')
        .next()
        .unwrap()
        .rsplit('/')
        .next()
        .unwrap()
        .starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let mut tx = pool.begin().await.unwrap();
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
    let (api, server) = start(&pool).await;
    let own_projects = value(
        api.request(Method::GET, &owner, owner.personal, "projects"),
        StatusCode::OK,
    )
    .await;
    let foreign_project = &own_projects[0]["id"];
    let project_path = format!("projects/{}", foreign_project.as_str().unwrap());
    for (user, elsewhere) in [
        (&editor, owner.personal),
        (&owner, team),
        (&editor, editor.personal),
    ] {
        value(
            api.request(Method::GET, user, elsewhere, &project_path),
            StatusCode::NOT_FOUND,
        )
        .await;
    }
    value(api.request(Method::POST,&editor,team,"people").json(&json!({
        "name":"Cross-space link", "projects":[{"id":foreign_project,"comment":"Shared work"}]
    })),StatusCode::NOT_FOUND).await;

    let person = value(
        api.request(Method::POST, &editor, team, "people")
            .json(&json!({
                "name":"Shared colleague","bornOn":"1980-06-01"
            })),
        StatusCode::CREATED,
    )
    .await;
    let path = format!("people/{}", person["id"].as_str().unwrap());
    let address = format!("{path}/addresses");
    value(
        api.request(Method::POST, &editor, team, &address)
            .json(&json!({
                "kind":"home","label":"Home","value":"Private street 19"
            })),
        StatusCode::CREATED,
    )
    .await;
    let note = format!("{path}/notes");
    value(
        api.request(Method::POST, &owner, team, &note)
            .json(&json!({"body":"Personal note by owner"})),
        StatusCode::CREATED,
    )
    .await;
    assert_eq!(
        value(
            api.request(Method::GET, &editor, team, &note),
            StatusCode::OK
        )
        .await,
        json!([])
    );
    let visible = value(
        api.request(Method::GET, &owner, team, &path),
        StatusCode::OK,
    )
    .await;
    assert_eq!(visible["name"], "Shared colleague");
    assert!(visible["bornOn"].is_null());
    assert_eq!(visible["addresses"], json!([]));
    value(
        api.request(Method::PATCH, &owner, team, &path)
            .json(&json!({"name":"Updated colleague"})),
        StatusCode::OK,
    )
    .await;
    let private = value(
        api.request(Method::GET, &editor, team, &path),
        StatusCode::OK,
    )
    .await;
    assert!(private["bornOn"]
        .as_str()
        .unwrap()
        .starts_with("1980-06-01"));
    assert_eq!(private["addresses"][0]["value"], "Private street 19");
    value(
        api.request(Method::DELETE, &owner, team, &path),
        StatusCode::CONFLICT,
    )
    .await;
    value(
        api.request(Method::GET, &editor, team, "backup"),
        StatusCode::FORBIDDEN,
    )
    .await;
    let archive = value(
        api.request(Method::GET, &owner, team, "backup"),
        StatusCode::OK,
    )
    .await;
    assert_eq!(archive["workspaceKind"], "team");
    for table in [
        "person_notes",
        "person_addresses",
        "person_birthdays",
        "schedule_settings",
    ] {
        assert_eq!(archive["tables"][table], json!([]));
    }
    assert!(!archive.to_string().contains("1980-06-01"));
    let restore = "restore?mode=replace";
    assert_eq!(
        api.request(Method::POST, &owner, team, restore)
            .json(&archive)
            .send()
            .await
            .unwrap()
            .status(),
        StatusCode::NO_CONTENT
    );
    assert_eq!(
        value(
            api.request(Method::GET, &editor, team, &path),
            StatusCode::OK
        )
        .await,
        private
    );
    let mut destructive = archive.clone();
    destructive["tables"]["people"] = json!([]);
    value(
        api.request(Method::POST, &owner, team, restore)
            .json(&destructive),
        StatusCode::CONFLICT,
    )
    .await;
    assert_eq!(
        value(
            api.request(Method::GET, &owner, team, &note),
            StatusCode::OK
        )
        .await[0]["body"],
        "Personal note by owner"
    );
    value(
        api.request(Method::POST, &owner, owner.personal, restore)
            .json(&archive),
        StatusCode::CONFLICT,
    )
    .await;
    let mut missing_manual = archive.clone();
    missing_manual["tables"]["sources"] = json!([]);
    value(
        api.request(Method::POST, &owner, team, restore)
            .json(&missing_manual),
        StatusCode::BAD_REQUEST,
    )
    .await;

    // A connected wearable occupies the same free slot as a task provider.
    sqlx::query("INSERT INTO sources(workspace_id,kind,name) VALUES($1,'whoop','Wearable')")
        .bind(owner.personal)
        .execute(&pool)
        .await
        .unwrap();
    let source = json!({"kind":"todoist","name":"Planner","projectId":foreign_project});
    let attempt = || {
        api.request(Method::POST, &owner, owner.personal, "sources")
            .json(&source)
            .send()
    };
    let (first, second) = tokio::join!(attempt(), attempt());
    let mut statuses = [
        first.unwrap().status().as_u16(),
        second.unwrap().status().as_u16(),
    ];
    statuses.sort();
    assert_eq!(statuses, [201, 409]);
    let free = value(
        api.request(Method::GET, &owner, owner.personal, "me"),
        StatusCode::OK,
    )
    .await;
    assert_eq!(free["sourceCount"], 2);
    assert_eq!(free["capabilities"]["externalSourceLimit"], 2);
    assert_eq!(free["capabilities"]["canAnalyze"], false);
    assert_eq!(free["capabilities"]["scheduleDays"], 1);
    let analysis_fields = ["estimateBuffer", "goldenHours", "energyAware"];
    let request_settings = json!({"estimateBuffer":true,"goldenHours":true,"energyAware":true});
    let free_settings = value(
        api.request(Method::PUT, &owner, owner.personal, "schedule/settings")
            .json(&request_settings),
        StatusCode::OK,
    )
    .await;
    let paid_settings = value(
        api.request(Method::PUT, &owner, team, "schedule/settings")
            .json(&request_settings),
        StatusCode::OK,
    )
    .await;
    for field in analysis_fields {
        assert_eq!(free_settings[field], false);
        assert_eq!(paid_settings[field], true);
    }
    let team_projects = value(
        api.request(Method::GET, &owner, team, "projects"),
        StatusCode::OK,
    )
    .await;
    let provider = json!({"kind":"todoist","name":"Team tasks","projectId":team_projects[0]["id"]});
    value(
        api.request(Method::POST, &editor, team, "sources")
            .json(&provider),
        StatusCode::FORBIDDEN,
    )
    .await;
    for _ in 0..3 {
        value(
            api.request(Method::POST, &owner, team, "sources")
                .json(&provider),
            StatusCode::CREATED,
        )
        .await;
    }
    let paid = value(
        api.request(Method::GET, &editor, team, "me"),
        StatusCode::OK,
    )
    .await;
    assert!(paid["capabilities"]["externalSourceLimit"].is_null());
    assert_eq!(paid["capabilities"]["canAnalyze"], true);
    sqlx::query("UPDATE workspaces SET plan='free' WHERE id=$1")
        .bind(team)
        .execute(&pool)
        .await
        .unwrap();
    value(
        api.request(Method::POST, &editor, team, "people")
            .json(&json!({"name":"No write access"})),
        StatusCode::FORBIDDEN,
    )
    .await;
    assert_eq!(
        value(
            api.request(Method::GET, &editor, team, &path),
            StatusCode::OK
        )
        .await["name"],
        "Updated colleague"
    );
    server.abort();
    let mut tx = pool.begin().await.unwrap();
    let spaces = [team, owner.personal, editor.personal];
    for table in ["schedule_settings", "people", "sources", "projects"] {
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
