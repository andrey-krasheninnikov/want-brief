use chrono::{DateTime, Duration, Utc};
use reqwest::{Method, RequestBuilder, StatusCode};
use serde_json::json;
use sqlx::PgPool;
use uuid::Uuid;

#[path = "support/workspaces.rs"]
mod workspaces;
use workspaces::{account, space, start, value, Account, Api};

impl Api {
    fn schedule(
        &self,
        account: &Account,
        space: Uuid,
        view: &str,
        from: DateTime<Utc>,
        to: DateTime<Utc>,
    ) -> RequestBuilder {
        self.request(Method::GET, account, space, "schedule")
            .query(&[
                ("view", view.to_owned()),
                ("from", from.to_rfc3339()),
                ("to", to.to_rfc3339()),
            ])
    }
}

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn plans_follow_assignments_and_share_only_team_data_and_daily_totals() {
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
    let other_team = space(&mut tx, editor.id, "team", "free").await;
    sqlx::query("INSERT INTO workspace_members(workspace_id,user_id,role) VALUES($1,$2,'editor')")
        .bind(team)
        .bind(editor.id)
        .execute(&mut *tx)
        .await
        .unwrap();
    tx.commit().await.unwrap();
    let (api, server) = start(&control).await;
    for (user, workspace) in [
        (&owner, owner.personal),
        (&owner, team),
        (&editor, editor.personal),
        (&editor, other_team),
    ] {
        value(api.request(Method::PUT,user,workspace,"schedule/settings").json(&json!({
            "timezone":"UTC","workdays":[0,1,2,3,4,5,6],"workStartMin":480,"workEndMin":1020,"meetingBufferMin":0,
        })), StatusCode::OK).await;
    }
    let today = Utc::now()
        .date_naive()
        .and_hms_opt(0, 0, 0)
        .unwrap()
        .and_utc();
    let tomorrow = today + Duration::days(1);
    let item = value(
        api.request(Method::POST, &owner, team, "items")
            .json(&json!({
                "kind":"task","title":"Prepare agenda","assigneeUserId":editor.id,
            })),
        StatusCode::CREATED,
    )
    .await;
    assert_eq!(item["workspaceId"], json!(team));
    assert_eq!(item["assigneeUserId"], json!(editor.id));
    let task = format!("items/{}", item["id"].as_str().unwrap());
    let edit = api
        .request(Method::PATCH, &owner, team, &task)
        .json(&json!({"status":"to_do"}));
    assert_eq!(
        value(edit, StatusCode::OK).await["assigneeUserId"],
        json!(editor.id)
    );
    value(
        api.request(Method::PATCH, &owner, team, &task)
            .json(&json!({"assigneeUserId":Uuid::new_v4()})),
        StatusCode::NOT_FOUND,
    )
    .await;

    let mut private_items = Vec::new();
    for (workspace, title) in [
        (editor.personal, "Personal errand"),
        (other_team, "Other team task"),
    ] {
        let created = value(
            api.request(Method::POST, &editor, workspace, "items")
                .json(&json!({
                    "title":title,"kind":"task","assigneeUserId":editor.id,
                })),
            StatusCode::CREATED,
        )
        .await;
        let path = format!("items/{}", created["id"].as_str().unwrap());
        value(
            api.request(Method::PATCH, &editor, workspace, &path)
                .json(&json!({"status":"to_do"})),
            StatusCode::OK,
        )
        .await;
        private_items.push(created);
    }
    let my_plan = value(
        api.schedule(&editor, team, "personal", today, tomorrow),
        StatusCode::OK,
    )
    .await;
    assert!(my_plan["unplanned"]
        .as_array()
        .unwrap()
        .iter()
        .any(|row| row["item"]["id"] == item["id"] && row["item"]["workspaceId"] == json!(team)));
    for private in &private_items {
        assert!(my_plan["unplanned"]
            .as_array()
            .unwrap()
            .iter()
            .any(|row| row["item"]["workspaceId"] == private["workspaceId"]
                && row["item"]["id"] == private["id"]));
    }
    let owner_plan = value(
        api.schedule(&owner, team, "personal", today, tomorrow),
        StatusCode::OK,
    )
    .await;
    assert!(!owner_plan
        .to_string()
        .contains(item["id"].as_str().unwrap()));
    let next = tomorrow + Duration::days(1);
    let free_team = value(
        api.request(Method::GET, &editor, other_team, "schedule")
            .query(&[("view", "team")]),
        StatusCode::OK,
    )
    .await;
    assert_eq!(free_team["from"], json!(today));
    assert_eq!(free_team["to"], json!(tomorrow));
    value(
        api.schedule(&editor, other_team, "team", tomorrow, next),
        StatusCode::FORBIDDEN,
    )
    .await;
    value(
        api.schedule(&owner, other_team, "team", today, tomorrow),
        StatusCode::NOT_FOUND,
    )
    .await;

    value(
        api.schedule(&editor, team, "personal", tomorrow, next),
        StatusCode::FORBIDDEN,
    )
    .await;
    value(
        api.request(Method::POST, &editor, team, "schedule/whatif")
            .json(&json!({"from":tomorrow,"to":next})),
        StatusCode::FORBIDDEN,
    )
    .await;

    let event = |title: &str, hour: i64, people: Vec<Uuid>| {
        json!({
            "title": title, "timezone":"UTC", "startsAt":tomorrow+Duration::hours(hour), "durationSeconds":3600,
            "canSkip":true,"participantUserIds":people,
        })
    };
    let private = value(
        api.request(Method::POST, &editor, editor.personal, "events")
            .json(&event("Private appointment", 9, vec![])),
        StatusCode::CREATED,
    )
    .await;
    let selected = value(
        api.request(Method::POST, &owner, team, "events")
            .json(&event("Design review", 11, vec![editor.id, editor.id])),
        StatusCode::CREATED,
    )
    .await;
    assert_eq!(selected["participantUserIds"], json!([editor.id]));
    value(
        api.request(Method::POST, &owner, team, "events")
            .json(&event("Open office", 12, vec![])),
        StatusCode::CREATED,
    )
    .await;
    value(
        api.request(Method::POST, &owner, team, "events")
            .json(&event("Invalid invitation", 15, vec![Uuid::new_v4()])),
        StatusCode::NOT_FOUND,
    )
    .await;

    value(api.request(Method::PATCH,&owner,team,&task).json(&json!({
        "plannedSeconds":3600,"pinnedAt":tomorrow+Duration::hours(14),"devDueAt":tomorrow+Duration::hours(17),
    })),StatusCode::OK).await;
    let review = value(
        api.request(Method::POST, &owner, team, "items")
            .json(&json!({"kind":"task","title":"Review tomorrow","assigneeUserId":editor.id})),
        StatusCode::CREATED,
    )
    .await;
    let review_path = format!("items/{}", review["id"].as_str().unwrap());
    value(
        api.request(Method::PATCH, &editor, team, &review_path)
            .json(&json!({"status":"review","reviewDueAt":tomorrow+Duration::hours(17)})),
        StatusCode::OK,
    )
    .await;
    let report = value(
        api.schedule(&owner, team, "team", tomorrow, next),
        StatusCode::OK,
    )
    .await;
    assert_eq!(report["timezone"], "UTC");
    assert_eq!(report["events"].as_array().unwrap().len(), 2);
    assert_eq!(
        report["events"][0]["originalOn"],
        json!(tomorrow.date_naive())
    );
    assert_eq!(report["items"][0]["assigneeUserId"], json!(editor.id));
    let load = |id: Uuid| {
        report["members"]
            .as_array()
            .unwrap()
            .iter()
            .find(|member| member["userId"] == json!(id))
            .unwrap()["days"][0]
            .clone()
    };
    assert_eq!(
        load(owner.id),
        json!({"date":tomorrow.date_naive(),"availableSeconds":32400,"plannedSeconds":0})
    );
    assert_eq!(
        load(editor.id),
        json!({"date":tomorrow.date_naive(),"availableSeconds":20400,"plannedSeconds":8400})
    );
    assert!(!report.to_string().contains(private["id"].as_str().unwrap()));
    assert!(!report.to_string().contains("Private appointment"));
    for private in &private_items {
        assert!(!report.to_string().contains(private["id"].as_str().unwrap()));
        assert!(!report
            .to_string()
            .contains(private["title"].as_str().unwrap()));
    }
    assert!(report.get("atRisk").is_none());
    let narrow = value(
        api.schedule(
            &owner,
            team,
            "team",
            tomorrow + Duration::hours(9),
            tomorrow + Duration::minutes(555),
        ),
        StatusCode::OK,
    )
    .await;
    let followup = value(
        api.schedule(&editor, team, "team", tomorrow, next)
            .query(&[("kind", "followup")]),
        StatusCode::OK,
    )
    .await;
    assert_eq!(followup["members"], report["members"]);
    assert_eq!(narrow["members"], report["members"]);
    assert_eq!(narrow["events"], json!([]));

    let four_weeks = today + Duration::days(28);
    let all_days = value(
        api.request(Method::GET, &editor, team, "schedule")
            .query(&[("view", "team")]),
        StatusCode::OK,
    )
    .await;
    assert_eq!(all_days["from"], json!(today));
    assert_eq!(all_days["to"], json!(four_weeks));
    assert_eq!(all_days["members"][0]["days"].as_array().unwrap().len(), 28);

    value(
        api.schedule(
            &editor,
            team,
            "team",
            four_weeks - Duration::hours(1),
            four_weeks + Duration::days(1),
        ),
        StatusCode::OK,
    )
    .await;
    value(
        api.schedule(
            &editor,
            team,
            "team",
            four_weeks,
            four_weeks + Duration::days(1),
        ),
        StatusCode::FORBIDDEN,
    )
    .await;
    let clear = value(
        api.request(Method::PATCH, &owner, team, &task)
            .json(&json!({"assigneeUserId":null})),
        StatusCode::OK,
    );
    let rename = value(
        api.request(Method::PATCH, &editor, team, &task)
            .json(&json!({"title":"Published agenda"})),
        StatusCode::OK,
    );
    tokio::join!(clear, rename);
    let updated = value(
        api.request(Method::GET, &editor, team, &task),
        StatusCode::OK,
    )
    .await;
    assert!(updated["assigneeUserId"].is_null());
    assert_eq!(updated["title"], "Published agenda");
    assert!(!value(
        api.schedule(&editor, team, "personal", today, tomorrow),
        StatusCode::OK
    )
    .await
    .to_string()
    .contains(item["id"].as_str().unwrap()));

    server.abort();
    let mut tx = control.begin().await.unwrap();
    let spaces = [owner.personal, editor.personal, team, other_team];
    for table in [
        "schedule_snapshots",
        "schedule_settings",
        "events",
        "items",
        "sources",
        "projects",
    ] {
        sqlx::query(&format!("DELETE FROM {table} WHERE workspace_id=ANY($1)"))
            .bind(spaces.as_slice())
            .execute(&mut *tx)
            .await
            .unwrap();
    }
    sqlx::query("DELETE FROM workspaces WHERE id=ANY($1)")
        .bind(spaces.as_slice())
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
