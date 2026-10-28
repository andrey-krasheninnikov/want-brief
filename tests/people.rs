use serde_json::{json, Value};
use sqlx::PgPool;
use std::sync::Arc;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn people_routes_preserve_project_links_and_age() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let token = auth.login("sample-password").await.unwrap().unwrap();
    let project_id: Uuid = sqlx::query_scalar("SELECT id FROM projects WHERE name='Work' LIMIT 1")
        .fetch_one(&pool)
        .await
        .unwrap();
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
    let response = client.post(format!("{root}/api/people"))
        .header("Cookie", &cookie)
        .json(&json!({"name":"  Ada  ","bornOn":"1990-01-02","projects":[{"id":project_id,"comment":" lead "}]}))
        .send().await.unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let person: Value = response.json().await.unwrap();
    assert_eq!(person["name"], "Ada");
    assert_eq!(
        person["projects"],
        json!([{"id":project_id,"comment":"lead"}])
    );
    assert!(person["age"].as_i64().unwrap() >= 35);
    assert_eq!(person["contacts"], json!([]));
    let person_id = person["id"].as_str().unwrap();
    let contact: Value = client
        .post(format!("{root}/api/people/{person_id}/contacts"))
        .header("Cookie", &cookie)
        .json(&json!({"kind":"telegram","label":"Chat","value":"@ada"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(contact["value"], "@ada");
    let address: Value = client
        .post(format!("{root}/api/people/{person_id}/addresses"))
        .header("Cookie", &cookie)
        .json(&json!({"kind":"home","label":"Home","value":"London"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(address["value"], "London");
    let site: Value = client
        .post(format!("{root}/api/people/{person_id}/sites"))
        .header("Cookie", &cookie)
        .json(&json!({"kind":"personal_site","url":"https://example.com/ada"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(site["kind"], "personal_site");
    let absence: Value = client
        .post(format!("{root}/api/people/{person_id}/absences"))
        .header("Cookie", &cookie)
        .json(&json!({"startsOn":"2026-11-01","endsOn":"2026-11-03"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(absence["startsOn"], "2026-11-01T00:00:00+00:00");
    let profession: Value = client
        .post(format!("{root}/api/people/{person_id}/professions"))
        .header("Cookie", &cookie)
        .json(&json!({"title":"Mathematician","monthlySalaryUsd":100.0,"monthlySalaryRub":0.0}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(profession["title"], "Mathematician");
    assert_eq!(profession["salaries"][0]["monthlySalaryUsd"], 100.0);
    let profession_id = profession["id"].as_str().unwrap();
    let updated_profession: Value = client
        .patch(format!(
            "{root}/api/people/{person_id}/professions/{profession_id}"
        ))
        .header("Cookie", &cookie)
        .json(&json!({"title":"Mathematician","monthlySalaryUsd":200.0,"monthlySalaryRub":0.0}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(updated_profession["salaries"].as_array().unwrap().len(), 2);
    let bond: Value = client
        .post(format!("{root}/api/people/{person_id}/bonds"))
        .header("Cookie", &cookie)
        .json(&json!({"kind":"friend","comment":"met at work"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(bond["events"][0]["action"], "open");
    let bond_id = bond["id"].as_str().unwrap();
    let changed: Value = client
        .patch(format!("{root}/api/people/{person_id}/bonds/{bond_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"kind":"comrade","actionComment":"closer"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(changed["events"].as_array().unwrap().len(), 2);
    let ended: Value = client
        .post(format!("{root}/api/people/{person_id}/bonds/{bond_id}/end"))
        .header("Cookie", &cookie)
        .json(&json!({"comment":"closed"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(ended["events"].as_array().unwrap().len(), 3);
    let note: Value = client
        .post(format!("{root}/api/people/{person_id}/notes"))
        .header("Cookie", &cookie)
        .json(&json!({"body":"Met today"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(note["body"], "Met today");
    let response = client
        .get(format!("{root}/api/people/{person_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let dossier: Value = response.json().await.unwrap();
    assert_eq!(dossier["contacts"][0]["id"], contact["id"]);
    assert_eq!(dossier["addresses"][0]["id"], address["id"]);
    assert_eq!(dossier["sites"][0]["id"], site["id"]);
    assert_eq!(dossier["absences"][0]["id"], absence["id"]);
    assert_eq!(dossier["professions"][0]["id"], profession["id"]);
    assert!(dossier["meBond"].is_null());
    assert!(dossier["lastNoteAt"].is_string());
    let response = client
        .patch(format!("{root}/api/people/{person_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"name":"Ada Lovelace","bornOn":"1990-01-02"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let person: Value = response.json().await.unwrap();
    assert_eq!(person["name"], "Ada Lovelace");
    assert_eq!(person["projects"], json!([]));
    let response = client
        .delete(format!("{root}/api/people/{person_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    server.abort();
}
