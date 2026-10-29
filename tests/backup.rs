use serde_json::{json, Value};
use sqlx::PgPool;
use std::path::Path;
use std::sync::Arc;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};
use want_brief::migrate::migrate;

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn backup_restores_records_and_relations_without_credentials_or_duplicates() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_issue3_"));
    migrate(&url, Path::new("migrations")).await.unwrap();
    let pool = PgPool::connect(&url).await.unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
    let token_box = Arc::new(
        TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff").unwrap(),
    );
    let project_id = Uuid::new_v4();
    let source_id = Uuid::new_v4();
    let item_id = Uuid::new_v4();
    let person_id = Uuid::new_v4();
    let note_id = Uuid::new_v4();
    let event_id = Uuid::new_v4();
    let company_id = Uuid::new_v4();
    sqlx::query("INSERT INTO projects (id,name) VALUES ($1,'Backup project')")
        .bind(project_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("INSERT INTO sources (id,kind,name,base_url,token_sealed,project_id,connected) VALUES ($1,'todoist','Backup source','https://api.todoist.com',$2,$3,true)")
        .bind(source_id)
        .bind(token_box.seal("private-token").unwrap())
        .bind(project_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("INSERT INTO items (id,source_id,title,status,kind,project_id) VALUES ($1,$2,'Backup item','backlog','task',$3)")
        .bind(item_id)
        .bind(source_id)
        .bind(project_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("INSERT INTO people (id,name) VALUES ($1,'Backup person')")
        .bind(person_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("INSERT INTO person_items (person_id,item_id) VALUES ($1,$2)")
        .bind(person_id)
        .bind(item_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("INSERT INTO item_notes (id,item_id,body) VALUES ($1,$2,'Backup note')")
        .bind(note_id)
        .bind(item_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query(
        "INSERT INTO events (id,title,kind,starts_at) VALUES ($1,'Backup event','event',now())",
    )
    .bind(event_id)
    .execute(&pool)
    .await
    .unwrap();
    sqlx::query("INSERT INTO event_notes (series_id,original_on,body) VALUES ($1,current_date,'Event note')")
        .bind(event_id).execute(&pool).await.unwrap();
    sqlx::query("INSERT INTO companies (id,name) VALUES ($1,'Backup company')")
        .bind(company_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("INSERT INTO company_people (company_id,person_id) VALUES ($1,$2)")
        .bind(company_id)
        .bind(person_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("INSERT INTO company_events (company_id,event_id) VALUES ($1,$2)")
        .bind(company_id)
        .bind(event_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("INSERT INTO schedule_settings (id,data) VALUES (1,'{\"timezone\":\"Europe/Moscow\"}'::jsonb) ON CONFLICT (id) DO UPDATE SET data=EXCLUDED.data")
        .execute(&pool).await.unwrap();

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
    assert_eq!(
        client
            .get(format!("{root}/api/backup"))
            .send()
            .await
            .unwrap()
            .status(),
        reqwest::StatusCode::UNAUTHORIZED
    );
    let response = client
        .get(format!("{root}/api/backup"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert_eq!(response.headers()["cache-control"], "no-store");
    let original = response.bytes().await.unwrap();
    let backup: Value = serde_json::from_slice(&original).unwrap();
    assert_eq!(backup["formatVersion"], 1);
    assert_eq!(backup["schemaVersion"], 47);
    let tables: Vec<String> = sqlx::query_scalar("SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename NOT IN ('users','sessions','goose_db_version') ORDER BY tablename")
        .fetch_all(&pool).await.unwrap();
    let exported: Vec<String> = backup["tables"]
        .as_object()
        .unwrap()
        .keys()
        .cloned()
        .collect();
    assert_eq!(exported, tables);
    assert!(backup["tables"]["sources"]
        .as_array()
        .unwrap()
        .iter()
        .all(|row| row.get("token_sealed").is_none()));
    assert!(!String::from_utf8_lossy(&original).contains("private-token"));
    assert_eq!(
        backup["tables"]["person_items"]
            .as_array()
            .unwrap()
            .iter()
            .filter(|row| row["item_id"] == item_id.to_string())
            .count(),
        1
    );

    let marker_id = Uuid::new_v4();
    sqlx::query("INSERT INTO notes (id,body) VALUES ($1,'Existing data')")
        .bind(marker_id)
        .execute(&pool)
        .await
        .unwrap();
    let mut invalid = backup.clone();
    invalid["tables"]["person_items"]
        .as_array_mut()
        .unwrap()
        .iter_mut()
        .find(|row| row["item_id"] == item_id.to_string())
        .unwrap()["item_id"] = json!(Uuid::new_v4());
    let response = client
        .post(format!("{root}/api/restore?mode=replace"))
        .header("Cookie", &cookie)
        .json(&invalid)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::BAD_REQUEST);
    let mut incompatible = backup.clone();
    incompatible["schemaVersion"] = json!(46);
    let response = client
        .post(format!("{root}/api/restore?mode=replace"))
        .header("Cookie", &cookie)
        .json(&incompatible)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CONFLICT);
    let mut incomplete = backup.clone();
    incomplete["tables"]["items"]
        .as_array_mut()
        .unwrap()
        .iter_mut()
        .find(|row| row["id"] == item_id.to_string())
        .unwrap()
        .as_object_mut()
        .unwrap()
        .remove("description");
    let response = client
        .post(format!("{root}/api/restore?mode=replace"))
        .header("Cookie", &cookie)
        .json(&incomplete)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::BAD_REQUEST);
    let response = client
        .post(format!("{root}/api/restore"))
        .header("Cookie", &cookie)
        .json(&backup)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::BAD_REQUEST);
    let marker: i64 = sqlx::query_scalar("SELECT count(*) FROM notes WHERE id=$1")
        .bind(marker_id)
        .fetch_one(&pool)
        .await
        .unwrap();
    assert_eq!(marker, 1);

    for _ in 0..2 {
        let response = client
            .post(format!("{root}/api/restore?mode=replace"))
            .header("Cookie", &cookie)
            .header("Content-Type", "application/json")
            .body(original.clone())
            .send()
            .await
            .unwrap();
        assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    }
    let restored: Value = client
        .get(format!("{root}/api/backup"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(restored["tables"], backup["tables"]);
    let marker: i64 = sqlx::query_scalar("SELECT count(*) FROM notes WHERE id=$1")
        .bind(marker_id)
        .fetch_one(&pool)
        .await
        .unwrap();
    assert_eq!(marker, 0);
    let linked: i64 =
        sqlx::query_scalar("SELECT count(*) FROM person_items WHERE person_id=$1 AND item_id=$2")
            .bind(person_id)
            .bind(item_id)
            .fetch_one(&pool)
            .await
            .unwrap();
    assert_eq!(linked, 1);
    let notes: i64 =
        sqlx::query_scalar("SELECT count(*) FROM item_notes WHERE id=$1 AND item_id=$2")
            .bind(note_id)
            .bind(item_id)
            .fetch_one(&pool)
            .await
            .unwrap();
    assert_eq!(notes, 1);
    let company_links: i64 = sqlx::query_scalar(
        "SELECT count(*) FROM company_people WHERE company_id=$1 AND person_id=$2",
    )
    .bind(company_id)
    .bind(person_id)
    .fetch_one(&pool)
    .await
    .unwrap();
    assert_eq!(company_links, 1);
    let event_links: i64 = sqlx::query_scalar(
        "SELECT count(*) FROM company_events WHERE company_id=$1 AND event_id=$2",
    )
    .bind(company_id)
    .bind(event_id)
    .fetch_one(&pool)
    .await
    .unwrap();
    assert_eq!(event_links, 1);
    let settings: Value = sqlx::query_scalar("SELECT data FROM schedule_settings WHERE id=1")
        .fetch_one(&pool)
        .await
        .unwrap();
    assert_eq!(settings["timezone"], "Europe/Moscow");
    let source: (String, bool) =
        sqlx::query_as("SELECT token_sealed, connected FROM sources WHERE id=$1")
            .bind(source_id)
            .fetch_one(&pool)
            .await
            .unwrap();
    assert_eq!(source, (String::new(), false));
    assert_eq!(
        client
            .get(format!("{root}/api/me"))
            .header("Cookie", &cookie)
            .send()
            .await
            .unwrap()
            .status(),
        reqwest::StatusCode::OK
    );
    server.abort();
}
