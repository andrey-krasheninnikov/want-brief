use serde_json::{json, Value};
use sqlx::PgPool;
use std::sync::Arc;
use uuid::Uuid;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn company_routes_preserve_links_and_career() {
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
    let person_id = Uuid::new_v4();
    sqlx::query("INSERT INTO people (id,name) VALUES ($1,'Test person')")
        .bind(person_id)
        .execute(&pool)
        .await
        .unwrap();
    let colleague_id = Uuid::new_v4();
    sqlx::query("INSERT INTO people (id,name) VALUES ($1,'Test colleague')")
        .bind(colleague_id)
        .execute(&pool)
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
    let response=client.post(format!("{root}/api/companies"))
        .header("Cookie",&cookie)
        .json(&json!({"name":"  Example Ltd  ","startedOn":"2020-01-15","projects":[{"id":project_id,"comment":"client"}],"people":[{"id":person_id,"comment":"lead"}]}))
        .send().await.unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::CREATED);
    let company: Value = response.json().await.unwrap();
    assert_eq!(company["name"], "Example Ltd");
    assert_eq!(
        company["projects"],
        json!([{"id":project_id,"comment":"client"}])
    );
    assert_eq!(
        company["people"],
        json!([{"id":person_id,"comment":"lead"}])
    );
    assert!(company["tenure"]["years"].as_i64().unwrap() >= 6);
    let company_id = company["id"].as_str().unwrap();
    let note: Value = client
        .post(format!("{root}/api/companies/{company_id}/notes"))
        .header("Cookie", &cookie)
        .json(&json!({"body":"First meeting"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(note["body"], "First meeting");
    let title: Value = client
        .post(format!("{root}/api/companies/{company_id}/titles"))
        .header("Cookie", &cookie)
        .json(&json!({"title":"Engineer","startedOn":"2020-01-15"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(title["titles"][0]["title"], "Engineer");
    let salary: Value = client
        .post(format!("{root}/api/companies/{company_id}/salaries"))
        .header("Cookie", &cookie)
        .json(&json!({"currency":"usd","amount":100.0}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(salary["salaries"][0]["amount"], 100.0);
    let changed_salary: Value = client
        .post(format!("{root}/api/companies/{company_id}/salaries"))
        .header("Cookie", &cookie)
        .json(&json!({"currency":"usd","amount":200.0,"comment":"raise"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(changed_salary["salaries"].as_array().unwrap().len(), 2);
    let manager: Value = client
        .post(format!("{root}/api/companies/{company_id}/manager"))
        .header("Cookie", &cookie)
        .json(&json!({"personId":person_id}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(manager["managers"][0]["personId"], person_id.to_string());
    let report: Value = client
        .post(format!("{root}/api/companies/{company_id}/reports"))
        .header("Cookie", &cookie)
        .json(&json!({"personId":colleague_id}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(report["reports"][0]["personId"], colleague_id.to_string());
    let report_id = report["reports"][0]["id"].as_str().unwrap();
    let ended_report: Value = client
        .delete(format!(
            "{root}/api/companies/{company_id}/reports/{report_id}"
        ))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert!(ended_report["reports"][0]["endedOn"].is_string());
    let contract: Value = client
        .post(format!("{root}/api/companies/{company_id}/contracts"))
        .header("Cookie", &cookie)
        .json(&json!({"personId":person_id,"kind":"labor"}))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(contract["contracts"][0]["kind"], "labor");
    let response = client
        .patch(format!("{root}/api/companies/{company_id}"))
        .header("Cookie", &cookie)
        .json(&json!({"name":"Example 2"}))
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let updated: Value = response.json().await.unwrap();
    assert_eq!(updated["people"], json!([]));
    let response = client
        .delete(format!("{root}/api/companies/{company_id}"))
        .header("Cookie", &cookie)
        .send()
        .await
        .unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::NO_CONTENT);
    sqlx::query("DELETE FROM people WHERE id=$1")
        .bind(person_id)
        .execute(&pool)
        .await
        .unwrap();
    sqlx::query("DELETE FROM people WHERE id=$1")
        .bind(colleague_id)
        .execute(&pool)
        .await
        .unwrap();
    server.abort();
}
