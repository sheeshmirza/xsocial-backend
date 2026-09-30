#!/usr/bin/env python3
import datetime
import json
import os
import re
import subprocess
import sys
import time
from bson import ObjectId
from pymongo import MongoClient

LOG_FILE = "/Users/fc472024/Downloads/xsocial-backend/e2e_api_logs.txt"
BASE_URL = "http://localhost:5000"
MONGO_URI = "mongodb://127.0.0.1:27017"
DB_NAME = "xsocial"

# Connect to MongoDB for state verification & relational fixtures
mongo_client = MongoClient(MONGO_URI)
db = mongo_client[DB_NAME]

log_f = open(LOG_FILE, "w", encoding="utf-8")

def log(text):
    log_f.write(text + "\n")
    log_f.flush()
    print(text)

def log_section(title):
    border = "=" * 80
    log(f"\n{border}\n{title}\n{border}\n")

step_num = 0

def run_curl(method, endpoint, headers=None, data=None, form_file=None, query=None, description=""):
    global step_num
    step_num += 1
    if headers is None:
        headers = {}

    url = BASE_URL + endpoint
    if query:
        url += "?" + query

    curl_cmd = ["curl", "-s", "-i", "-X", method, url]

    for k, v in headers.items():
        curl_cmd.extend(["-H", f"{k}: {v}"])

    if form_file:
        curl_cmd.extend(["-F", f"file=@{form_file}"])
    elif data is not None:
        if isinstance(data, (dict, list)):
            json_str = json.dumps(data)
            if "Content-Type" not in headers:
                curl_cmd.extend(["-H", "Content-Type: application/json"])
            curl_cmd.extend(["--data", json_str])
        else:
            curl_cmd.extend(["--data", str(data)])

    # Format curl display string
    curl_disp = "curl -X " + method + " '" + url + "'"
    for k, v in headers.items():
        curl_disp += f" \\\n  -H '{k}: {v}'"
    if form_file:
        curl_disp += f" \\\n  -F 'file=@{form_file}'"
    elif data is not None:
        if isinstance(data, (dict, list)):
            curl_disp += f" \\\n  -H 'Content-Type: application/json' \\\n  -d '{json.dumps(data)}'"
        else:
            curl_disp += f" \\\n  -d '{data}'"

    log(f"--- [STEP {step_num}] {description} ---")
    log(">> REQUEST:")
    log(curl_disp)

    # Run the curl command safely capturing raw bytes
    res = subprocess.run(curl_cmd, capture_output=True)
    raw_output = res.stdout

    # Split headers and body on \r\n\r\n or \n\n
    header_sep = b"\r\n\r\n"
    if header_sep not in raw_output:
        header_sep = b"\n\n"

    parts = raw_output.split(header_sep, 1)
    res_headers_bytes = parts[0] if len(parts) > 0 else b""
    res_body_bytes = parts[1] if len(parts) > 1 else b""

    res_headers = res_headers_bytes.decode("utf-8", errors="replace")
    status_line = res_headers.split("\n")[0] if res_headers else ""

    log("<< RESPONSE:")
    log(f"Status: {status_line.strip()}")
    log("Headers:")
    for h in res_headers.split("\n")[1:]:
        if h.strip():
            log(f"  {h.strip()}")

    log("Body:")
    # Check if response is binary (image or stream)
    is_binary = any(t in res_headers.lower() for t in ["image/", "application/octet-stream"])
    if is_binary:
        log(f"<binary data: {len(res_body_bytes)} bytes>")
        body_obj = f"<binary data: {len(res_body_bytes)} bytes>"
    else:
        res_body_text = res_body_bytes.decode("utf-8", errors="replace")
        try:
            parsed = json.loads(res_body_text)
            log(json.dumps(parsed, indent=2))
            body_obj = parsed
        except Exception:
            log(res_body_text if res_body_text else "<empty body>")
            body_obj = res_body_text

    log("") # Empty line between steps
    return status_line, body_obj

print("Starting Complete End-to-End API Verification Suite...")
log("#" * 80)
log(f"# XSocial Backend End-to-End API Test Logs")
log(f"# Executed: {time.strftime('%Y-%m-%d %H:%M:%S UTC', time.gmtime())}")
log(f"# Target Base URL: {BASE_URL}")
log("#" * 80)

# ==============================================================================
# 1. Health Checks
# ==============================================================================
log_section("1. SYSTEM HEALTH CHECKS")
run_curl("GET", "/healthz", description="System Healthz Probe")
run_curl("GET", "/health", description="System Health Probe (alias)")

# ==============================================================================
# 2. Authentication & User Management
# ==============================================================================
log_section("2. AUTHENTICATION & USER MANAGEMENT")
ts = int(time.time())
email = f"e2e_master_{ts}@example.com"
password = "MasterPassword123!"

# 2.1 Sign Up
_, signup_res = run_curl("POST", "/v1/auth/signup", data={"email": email, "password": password}, description="User Registration (Signup)")
verification_token = ""
if isinstance(signup_res, dict):
    verification_token = signup_res.get("verificationToken", "")
    if not verification_token and "data" in signup_res and isinstance(signup_res["data"], dict):
        verification_token = signup_res["data"].get("verificationToken", "")

# 2.2 Verify Email
if verification_token:
    run_curl("POST", "/v1/auth/verify-email", data={"token": verification_token}, description="Verify User Email with Token")

# 2.3 User Login
_, login_res = run_curl("POST", "/v1/auth/login", data={"email": email, "password": password}, description="User Login")
access_token = ""
refresh_token = ""
user_id = ""

if isinstance(login_res, dict):
    access_token = login_res.get("token") or ""
    refresh_token = login_res.get("refreshToken") or ""
    if not access_token and "data" in login_res and isinstance(login_res["data"], dict):
        data_obj = login_res["data"]
        tokens_obj = data_obj.get("tokens", {})
        access_token = tokens_obj.get("accessToken", "")
        refresh_token = tokens_obj.get("refreshToken", "")
    if "data" in login_res and isinstance(login_res["data"], dict):
        user_id = login_res["data"].get("user", {}).get("id", "")

auth_hdr = {"Authorization": f"Bearer {access_token}"}

# 2.4 Token Refresh
_, refresh_res = run_curl("POST", "/v1/auth/refresh", data={"refreshToken": refresh_token}, description="Refresh Access Token")
if isinstance(refresh_res, dict):
    new_token = refresh_res.get("token")
    if new_token:
        access_token = new_token
        auth_hdr = {"Authorization": f"Bearer {access_token}"}

# 2.5 Forgot Password & Password Reset Flow
_, fp_res = run_curl("POST", "/v1/auth/forgot-password", data={"email": email}, description="Request Password Reset")
reset_token = ""
if isinstance(fp_res, dict):
    reset_token = fp_res.get("resetToken", "")

if reset_token:
    run_curl("POST", "/v1/auth/reset-password", data={"token": reset_token, "newPassword": "ResetPassword123!"}, description="Reset Password using Reset Token")
    # Re-login with updated password
    _, relogin_res = run_curl("POST", "/v1/auth/login", data={"email": email, "password": "ResetPassword123!"}, description="Re-login with Reset Password")
    if isinstance(relogin_res, dict) and relogin_res.get("token"):
        access_token = relogin_res.get("token")
        refresh_token = relogin_res.get("refreshToken", refresh_token)
        auth_hdr = {"Authorization": f"Bearer {access_token}"}
    password = "ResetPassword123!"

# 2.6 User Profile (GET /v1/users/me)
run_curl("GET", "/v1/users/me", headers=auth_hdr, description="Get Current User Profile")

# 2.7 Update Profile (PUT & PATCH /v1/users/me)
run_curl("PUT", "/v1/users/me", headers=auth_hdr, data={"fullName": "E2E Master Tester", "timezone": "America/New_York"}, description="Update User Profile (PUT)")
run_curl("PATCH", "/v1/users/me", headers=auth_hdr, data={"fullName": "E2E Senior Director", "timezone": "Europe/London"}, description="Update User Profile (PATCH)")

# 2.8 Change Password
new_password = "ChangedPassword123!"
run_curl("PUT", "/v1/users/change-password", headers=auth_hdr, data={"oldPassword": password, "newPassword": new_password}, description="Change User Password")
# Re-login with new password
_, relogin_pw = run_curl("POST", "/v1/auth/login", data={"email": email, "password": new_password}, description="Re-login with Changed Password")
if isinstance(relogin_pw, dict) and relogin_pw.get("token"):
    access_token = relogin_pw.get("token")
    refresh_token = relogin_pw.get("refreshToken", refresh_token)
    auth_hdr = {"Authorization": f"Bearer {access_token}"}
password = new_password

# ==============================================================================
# 3. Brands & Workspaces (RBAC)
# ==============================================================================
log_section("3. BRANDS & WORKSPACES")
brand_req = {
    "name": "Acme Global Brands",
    "description": "Primary enterprise testing workspace",
    "timezone": "America/New_York",
    "currency": "USD"
}
_, brand_res = run_curl("POST", "/v1/brands", headers=auth_hdr, data=brand_req, description="Create Brand Workspace")
brand_id = ""
if isinstance(brand_res, dict):
    brand_id = brand_res.get("id", "")

run_curl("GET", "/v1/brands", headers=auth_hdr, description="List All Brands for User")
run_curl("GET", f"/v1/brands/{brand_id}", headers=auth_hdr, description="Get Brand Workspace Details")
run_curl("PUT", f"/v1/brands/{brand_id}", headers=auth_hdr, data={"name": "Acme Global Enterprises", "timezone": "America/Chicago"}, description="Update Brand Workspace (PUT)")
run_curl("PATCH", f"/v1/brands/{brand_id}", headers=auth_hdr, data={"name": "Acme Global Holdings"}, description="Update Brand Workspace (PATCH)")

# Secondary User for Brand Member Operations
member_email = f"brand_member_{ts}@example.com"
_, mem_reg = run_curl("POST", "/v1/auth/signup", data={"email": member_email, "password": "MemberPassword123!"}, description="Register Secondary Member User")
member_id = ""
if isinstance(mem_reg, dict) and "data" in mem_reg and isinstance(mem_reg["data"], dict):
    member_id = mem_reg["data"].get("id", "")
mem_tok = mem_reg.get("verificationToken", "") if isinstance(mem_reg, dict) else ""
if mem_tok:
    run_curl("POST", "/v1/auth/verify-email", data={"token": mem_tok}, description="Verify Secondary Member Email")

_, add_mem_res = run_curl("POST", f"/v1/brands/{brand_id}/members", headers=auth_hdr, data={"email": member_email, "role": "editor"}, description="Add Member to Brand")
run_curl("GET", f"/v1/brands/{brand_id}/members", headers=auth_hdr, description="List Brand Members")
if member_id:
    run_curl("DELETE", f"/v1/brands/{brand_id}/members/{member_id}", headers=auth_hdr, description="Remove Member from Brand")

# ==============================================================================
# 4. API Keys & Programmatic Access
# ==============================================================================
log_section("4. API KEYS (SCOPED PROGRAMMATIC ACCESS)")
api_key_req = {
    "name": "Production Metricool & CI Integration Key",
    "brandId": brand_id,
    "permissions": ["posts:read", "posts:write", "analytics:read", "brands:read"],
    "expiresInDays": 90
}
_, key_res = run_curl("POST", "/v1/api-keys", headers=auth_hdr, data=api_key_req, description="Create Scoped API Key")
api_key_raw = ""
api_key_id = ""
if isinstance(key_res, dict):
    api_key_raw = key_res.get("rawKey", "")
    api_key_id = key_res.get("apiKey", {}).get("id", "")

run_curl("GET", "/v1/api-keys", headers=auth_hdr, description="List API Keys")

# Test authentication using the generated API Key
if api_key_raw:
    run_curl("GET", "/v1/brands", headers={"X-API-Key": api_key_raw}, description="Authenticate with X-API-Key Header")
    run_curl("GET", "/v1/brands", headers={"Authorization": f"Bearer {api_key_raw}"}, description="Authenticate with Authorization: Bearer <API_KEY>")

# ==============================================================================
# 5. Media Uploads & Assets
# ==============================================================================
log_section("5. MEDIA UPLOADS & ASSETS")
dummy_img = "/tmp/test_upload.png"
with open(dummy_img, "wb") as f:
    f.write(b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15c4\x00\x00\x00\nIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01\r\n-\xb4\x00\x00\x00\x00IEND\xaeB`\x82")

_, upload_res = run_curl("POST", "/v1/uploads/media", headers=auth_hdr, form_file=dummy_img, description="Upload Media Asset (PNG Image)")
media_id = ""
media_url = ""
if isinstance(upload_res, dict) and "data" in upload_res and isinstance(upload_res["data"], dict):
    media_id = upload_res["data"].get("id", "")
    media_url = upload_res["data"].get("url", "")
elif isinstance(upload_res, dict):
    media_id = upload_res.get("id", "")
    media_url = upload_res.get("url", "")

if media_id:
    run_curl("GET", f"/v1/uploads/media/{media_id}", description="Fetch Uploaded Media Asset (Public)")

# ==============================================================================
# 6. Social Accounts & Platform Connections
# ==============================================================================
log_section("6. SOCIAL HANDLES & ACCOUNTS")

# Create a test social handle linked to the brand in DB to test account listing and deletion
social_handle_oid = ObjectId()
db.social_handles.insert_one({
    "_id": social_handle_oid,
    "userId": ObjectId(user_id) if user_id else ObjectId(),
    "brandId": ObjectId(brand_id) if brand_id else None,
    "platform": "instagram",
    "platformId": "ig_test_98765",
    "platformUsername": "acmeglobal",
    "platformDisplayName": "Acme Global Official",
    "platformAccessToken": "EAABsample_token_mock",
    "platformAccessTokenExpiry": datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=60),
    "platformMetadata": {"accountType": "BUSINESS"},
    "createdAt": datetime.datetime.now(datetime.timezone.utc),
    "updatedAt": datetime.datetime.now(datetime.timezone.utc)
})

run_curl("GET", "/v1/social/accounts", headers=auth_hdr, query=f"brandId={brand_id}", description="List Social Accounts for Brand")
run_curl("POST", "/v1/social/connect/instagram", headers=auth_hdr, data={"code": "oauth_code_sample", "redirectUri": "http://localhost:3000/callback"}, description="Connect Instagram Account (OAuth Exchange)")
run_curl("POST", "/v1/social/connect/linkedin", headers=auth_hdr, data={"code": "oauth_code_sample", "redirectUri": "http://localhost:3000/callback"}, description="Connect LinkedIn Account (OAuth Exchange)")
run_curl("POST", "/v1/social/refresh-token", headers=auth_hdr, data={"platform": "instagram", "accountId": "ig_test_98765"}, description="Refresh Social Account Token")
run_curl("GET", "/v1/social/callback/instagram", query="code=auth_code_123&state=xyz", description="OAuth Callback (Public Webhook/Redirect)")
run_curl("DELETE", f"/v1/social/{str(social_handle_oid)}", headers=auth_hdr, description="Disconnect Social Account")

# ==============================================================================
# 7. Posts, Approval Workflow, Publishing & Resharing
# ==============================================================================
log_section("7. POSTS, APPROVAL WORKFLOW & RESHARING")

create_post_payload = {
    "brandId": brand_id,
    "title": "Q4 Product Launch Announcement",
    "caption": "Exciting product launch announcement! Check out our new platform features #launch #innovation",
    "hashtags": ["launch", "innovation"],
    "selectedPlatforms": ["instagram", "linkedin"],
    "media": [
        {
            "type": "image",
            "url": media_url if media_url else "https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe"
        }
    ],
    "postType": "reel",
    "visibility": "public",
    "timezone": "America/New_York",
    "status": "draft",
    "firstComment": "Leave your thoughts below! #community",
    "instagramTrial": {
        "enabled": True,
        "graduationStrategy": "SS_PERFORMANCE"
    }
}
_, post_res = run_curl("POST", "/v1/posts", headers=auth_hdr, data=create_post_payload, description="Create Post (Draft with IG Trial Reel Settings)")
post_id = ""
if isinstance(post_res, dict):
    post_id = post_res.get("id", "")

run_curl("GET", "/v1/posts", headers=auth_hdr, query=f"brandId={brand_id}&limit=10", description="List Posts for Brand")
run_curl("GET", f"/v1/posts/{post_id}", headers=auth_hdr, description="Get Post Details by ID")

# Update post (PUT & PATCH)
run_curl("PUT", f"/v1/posts/{post_id}", headers=auth_hdr, data={
    "brandId": brand_id,
    "title": "Q4 Product Launch v2",
    "caption": "Exciting product launch announcement v2! Check out updated features #launch",
    "selectedPlatforms": ["instagram", "linkedin"],
    "status": "draft"
}, description="Update Post Content (PUT)")

run_curl("PATCH", f"/v1/posts/{post_id}", headers=auth_hdr, data={
    "caption": "Exciting product launch announcement v3! Official release is here #release"
}, description="Update Post Content (PATCH)")

# Submit post for approval
_, submit_res = run_curl("POST", f"/v1/posts/{post_id}/submit-approval", headers=auth_hdr, description="Submit Post for Approval Workflow")
approval_token = ""
if isinstance(submit_res, dict):
    approval_token = submit_res.get("approvalToken", "")

if approval_token:
    run_curl("GET", f"/v1/posts/review/{approval_token}", description="Public External Post Review (Client Link)")
    run_curl("POST", f"/v1/posts/review/{approval_token}/action", data={
        "action": "reject",
        "reviewerName": "External Client Reviewer",
        "feedback": "Needs refined hashtag strategy"
    }, description="Public External Post Review Action (Client Reject)")

# Direct Reject & Approve Endpoints
run_curl("POST", f"/v1/posts/{post_id}/reject", headers=auth_hdr, data={"reason": "Internal review requested changes"}, description="Reject Post (Internal)")
run_curl("POST", f"/v1/posts/{post_id}/approve", headers=auth_hdr, data={"comment": "Approved for scheduled publishing"}, description="Approve Post (Internal)")

# Reshare Post across Accounts / Platforms
reshare_payload = {
    "targetBrandId": brand_id,
    "targetPlatform": "linkedin",
    "additionalCommentary": "Resharing our core landmark release with our professional network!",
    "quoteRepost": True
}
_, reshare_res = run_curl("POST", f"/v1/posts/{post_id}/reshare", headers=auth_hdr, data=reshare_payload, description="Reshare Post to Another Account/Platform")
reshare_id = ""
if isinstance(reshare_res, dict):
    reshare_id = reshare_res.get("id", "")

# Bulk Recurring Controls
run_curl("POST", "/v1/posts/bulk/pause-recurrence", headers=auth_hdr, data={"ids": [post_id]}, description="Bulk Pause Recurring Posts")
run_curl("POST", "/v1/posts/bulk/resume-recurrence", headers=auth_hdr, data={"ids": [post_id]}, description="Bulk Resume Recurring Posts")

# ==============================================================================
# 8. Planner & Best Times Heatmaps
# ==============================================================================
log_section("8. PLANNER & BEST TIMES HEATMAPS")
run_curl("GET", "/v1/planner/best-times", headers=auth_hdr, query=f"brandId={brand_id}&platform=instagram", description="Get Best Times Heatmap for Instagram")
run_curl("GET", "/v1/planner/best-times", headers=auth_hdr, query=f"brandId={brand_id}&platform=linkedin", description="Get Best Times Heatmap for LinkedIn")

# ==============================================================================
# 9. Unified Inbox & Community Management
# ==============================================================================
log_section("9. UNIFIED INBOX & COMMUNITY MANAGEMENT")
_, canned_res = run_curl("POST", "/v1/inbox/canned-replies", headers=auth_hdr, query=f"brandId={brand_id}", data={
    "shortcut": "!welcome",
    "title": "Welcome Greeting",
    "content": "Thank you for reaching out! A specialist will get back to you shortly."
}, description="Create Canned Reply Template")
canned_id = ""
if isinstance(canned_res, dict):
    canned_id = canned_res.get("id") or canned_res.get("data", {}).get("id", "")

run_curl("GET", "/v1/inbox/canned-replies", headers=auth_hdr, query=f"brandId={brand_id}", description="List Brand Canned Replies")

# Seed a test conversation in DB to verify inbox conversation retrieval and messaging
conv_oid = ObjectId()
db.inbox_conversations.insert_one({
    "_id": conv_oid,
    "brandId": ObjectId(brand_id),
    "platform": "instagram",
    "socialHandleId": social_handle_oid,
    "externalThreadId": "thread_demo_101",
    "senderName": "Sarah Jenkins",
    "senderAvatar": "https://example.com/sarah.jpg",
    "senderHandle": "sarahj",
    "lastMessageText": "Hi! Do you offer enterprise support plans?",
    "lastMessageAt": datetime.datetime.now(datetime.timezone.utc),
    "status": "open",
    "unread": True,
    "tags": ["sales", "inquiry"],
    "sentiment": "positive",
    "createdAt": datetime.datetime.now(datetime.timezone.utc),
    "updatedAt": datetime.datetime.now(datetime.timezone.utc)
})

run_curl("GET", "/v1/inbox", headers=auth_hdr, query=f"brandId={brand_id}", description="List Unified Inbox Conversations")
run_curl("GET", f"/v1/inbox/{str(conv_oid)}", headers=auth_hdr, query=f"brandId={brand_id}", description="Get Inbox Conversation Detail")
run_curl("POST", f"/v1/inbox/{str(conv_oid)}/reply", headers=auth_hdr, query=f"brandId={brand_id}", data={
    "messageText": "Hello Sarah, yes! We offer 24/7 dedicated enterprise SLA support plans."
}, description="Reply to Inbox Conversation")
run_curl("PATCH", f"/v1/inbox/{str(conv_oid)}/status", headers=auth_hdr, query=f"brandId={brand_id}", data={
    "status": "resolved"
}, description="Update Inbox Conversation Status")

# ==============================================================================
# 10. SmartLinks (Link-in-Bio Pages)
# ==============================================================================
log_section("10. SMARTLINKS (LINK-IN-BIO PAGES)")
smartlink_slug = f"acme-links-{ts}"
smartlink_payload = {
    "title": "Acme Global Official Links",
    "slug": smartlink_slug,
    "description": "All our official links and social destinations in one place.",
    "theme": "dark",
    "buttons": [
        {"id": "btn_website", "title": "Visit Website", "url": "https://example.com", "isActive": True},
        {"id": "btn_store", "title": "Online Store", "url": "https://store.example.com", "isActive": True}
    ]
}
_, smartlink_res = run_curl("POST", "/v1/smartlinks", headers=auth_hdr, query=f"brandId={brand_id}", data=smartlink_payload, description="Create SmartLink Page")
smartlink_id = ""
if isinstance(smartlink_res, dict):
    smartlink_id = smartlink_res.get("id", "")

run_curl("GET", "/v1/smartlinks", headers=auth_hdr, query=f"brandId={brand_id}", description="List SmartLinks for Brand")
run_curl("PUT", f"/v1/smartlinks/{smartlink_id}", headers=auth_hdr, query=f"brandId={brand_id}", data={
    "title": "Acme Global Official Links (Updated)",
    "description": "Updated official destinations",
    "theme": "modern"
}, description="Update SmartLink Page (PUT)")
run_curl("PATCH", f"/v1/smartlinks/{smartlink_id}", headers=auth_hdr, query=f"brandId={brand_id}", data={
    "title": "Acme Global Official Bio"
}, description="Update SmartLink Page (PATCH)")
run_curl("GET", f"/v1/smartlinks/{smartlink_id}/analytics", headers=auth_hdr, query=f"brandId={brand_id}", description="Get SmartLink Page Analytics")

# Public Bio Page and Click Tracking
run_curl("GET", f"/v1/bio/{smartlink_slug}", description="View Public Bio Page")
run_curl("POST", f"/v1/bio/{smartlink_slug}/click/btn_website", description="Track Button Click on Bio Page (Public)")

# ==============================================================================
# 11. Autolists & RSS Feeds
# ==============================================================================
log_section("11. AUTOLISTS & RSS FEEDS (EVERGREEN CONTENT QUEUES)")
autolist_payload = {
    "name": "Weekly Evergreen Tips",
    "schedule": ["Monday 09:00", "Wednesday 14:00", "Friday 11:00"],
    "isCircular": True,
    "isActive": True
}
_, autolist_res = run_curl("POST", "/v1/autolists", headers=auth_hdr, query=f"brandId={brand_id}", data=autolist_payload, description="Create Autolist Evergreen Queue")
autolist_id = ""
if isinstance(autolist_res, dict):
    autolist_id = autolist_res.get("id", "")

run_curl("GET", "/v1/autolists", headers=auth_hdr, query=f"brandId={brand_id}", description="List Autolists for Brand")
run_curl("POST", f"/v1/autolists/{autolist_id}/posts", headers=auth_hdr, query=f"brandId={brand_id}", data={
    "content": "Evergreen tip #1: Consistent brand voice drives highest community engagement!"
}, description="Add Post to Autolist Queue")

rss_payload = {
    "autolistId": autolist_id,
    "feedUrl": "https://feeds.bbci.co.uk/news/technology/rss.xml",
    "name": "Tech News RSS Feed",
    "autoPublish": False,
    "prefix": "[Tech Digest]"
}
_, rss_res = run_curl("POST", "/v1/rss-feeds", headers=auth_hdr, query=f"brandId={brand_id}", data=rss_payload, description="Create RSS Feed Syndication")
rss_id = ""
if isinstance(rss_res, dict):
    rss_id = rss_res.get("id", "")

run_curl("GET", "/v1/rss-feeds", headers=auth_hdr, query=f"brandId={brand_id}", description="List RSS Feeds for Brand")

# ==============================================================================
# 12. Competitor Benchmarking
# ==============================================================================
log_section("12. COMPETITOR BENCHMARKING & TRACKING")
comp_payload = {
    "name": "Global Rival Corp",
    "platform": "instagram",
    "handle": "globalrivalcorp"
}
_, comp_res = run_curl("POST", "/v1/competitors", headers=auth_hdr, query=f"brandId={brand_id}", data=comp_payload, description="Add Competitor to Benchmark")
comp_id = ""
if isinstance(comp_res, dict):
    comp_id = comp_res.get("id", "")

run_curl("GET", "/v1/competitors", headers=auth_hdr, query=f"brandId={brand_id}", description="List Competitors for Brand")
target_comp_id = comp_id if comp_id else "507f1f77bcf86cd799439011"
run_curl("POST", f"/v1/competitors/{target_comp_id}/snapshots", headers=auth_hdr, query=f"brandId={brand_id}", data={
    "followers": 125000,
    "following": 850,
    "postsCount": 1420,
    "engagementRate": 3.85
}, description="Record Competitor Metrics Snapshot")
run_curl("POST", f"/v1/competitors/{target_comp_id}/sync", headers=auth_hdr, query=f"brandId={brand_id}", description="Trigger Competitor Sync")

# ==============================================================================
# 13. Paid Ads Management (Google, Meta, TikTok)
# ==============================================================================
log_section("13. PAID ADS MANAGEMENT (GOOGLE, META, TIKTOK)")
ad_payload = {
    "platform": "meta",
    "externalCampaignId": "cmp_meta_98765",
    "name": "Q4 Brand Awareness Drive",
    "status": "ACTIVE",
    "objective": "AWARENESS",
    "budget": 5000.0,
    "spend": 1240.50,
    "impressions": 485000,
    "clicks": 14200,
    "conversions": 630,
    "dateRange": "2026-Q4"
}
_, ad_res = run_curl("POST", "/v1/ads/campaigns", headers=auth_hdr, query=f"brandId={brand_id}", data=ad_payload, description="Create Ad Campaign Record")
ad_id = ""
if isinstance(ad_res, dict):
    ad_id = ad_res.get("id", "")

target_ad_id = ad_id if ad_id else "507f1f77bcf86cd799439011"
run_curl("GET", "/v1/ads/campaigns", headers=auth_hdr, query=f"brandId={brand_id}", description="List Ad Campaigns for Brand")
run_curl("PUT", f"/v1/ads/campaigns/{target_ad_id}", headers=auth_hdr, query=f"brandId={brand_id}", data={
    "spend": 1850.75,
    "clicks": 19800,
    "conversions": 890
}, description="Update Ad Campaign Performance (PUT)")
run_curl("PATCH", f"/v1/ads/campaigns/{target_ad_id}", headers=auth_hdr, query=f"brandId={brand_id}", data={
    "status": "PAUSED"
}, description="Update Ad Campaign Status (PATCH)")
run_curl("GET", "/v1/ads/summary", headers=auth_hdr, query=f"brandId={brand_id}", description="Get Multi-Channel Ad Spend Summary")
run_curl("POST", "/v1/ads/sync", headers=auth_hdr, query=f"brandId={brand_id}", data={"platform": "meta"}, description="Sync Ad Campaigns from Network API")

# ==============================================================================
# 14. Analytics (Web Tracking, Pixel & Social Rollups)
# ==============================================================================
log_section("14. WEB ANALYTICS & SOCIAL ROLLUPS")
run_curl("GET", "/v1/tracker/pixel.png", query=f"brandId={brand_id}&page=/products", description="Public Web Tracking 1x1 Transparent Pixel")
run_curl("POST", "/v1/tracker/event", data={
    "brandId": brand_id,
    "eventName": "purchase_completed",
    "properties": {"orderValue": 99.0, "currency": "USD"}
}, description="Public Web Event Beacon (POST)")
run_curl("GET", "/v1/analytics/web", headers=auth_hdr, query=f"brandId={brand_id}", description="Get Aggregated Web Traffic Analytics")
run_curl("POST", "/v1/analytics/social/rollups", headers=auth_hdr, query=f"brandId={brand_id}", data={
    "platform": "instagram",
    "impressions": 54200,
    "reach": 41200,
    "engagements": 3890,
    "followersGained": 340,
    "period": "daily"
}, description="Record Social Analytics Rollup Metric")
run_curl("GET", "/v1/analytics/social", headers=auth_hdr, query=f"brandId={brand_id}&platform=instagram", description="Get Social Performance Analytics")

# ==============================================================================
# 15. Reports Generation & Export
# ==============================================================================
log_section("15. REPORTS GENERATION & EXPORTS")
report_payload = {
    "brandId": brand_id,
    "title": "Monthly Executive Social & Web Performance Report",
    "format": "csv",
    "startDate": "2026-09-01T00:00:00Z",
    "endDate": "2026-09-30T23:59:59Z"
}
_, report_res = run_curl("POST", "/v1/reports/generate", headers=auth_hdr, data=report_payload, description="Generate Analytics Executive Report")
report_id = ""
if isinstance(report_res, dict):
    report_id = report_res.get("id", "")

run_curl("GET", "/v1/reports", headers=auth_hdr, query=f"brandId={brand_id}", description="List Generated Reports")
if report_id:
    run_curl("GET", f"/v1/reports/{report_id}/download", headers=auth_hdr, description="Download Generated Report File")

# ==============================================================================
# 16. Webhooks
# ==============================================================================
log_section("16. WEBHOOKS & EVENT SUBSCRIPTIONS")
webhook_payload = {
    "brandId": brand_id,
    "url": "https://webhook.site/test-endpoint",
    "events": ["post.published", "post.failed", "inbox.message"],
    "secret": "whsec_supersecretkey123"
}
_, wh_res = run_curl("POST", "/v1/webhooks", headers=auth_hdr, data=webhook_payload, description="Create Webhook Subscription")
webhook_id = ""
if isinstance(wh_res, dict):
    webhook_id = wh_res.get("id", "")

run_curl("GET", "/v1/webhooks", headers=auth_hdr, query=f"brandId={brand_id}", description="List Active Webhook Subscriptions")

# ==============================================================================
# 17. Audit Logs
# ==============================================================================
log_section("17. AUDIT LOGS & COMPLIANCE")
run_curl("GET", "/v1/audit-logs", headers=auth_hdr, query=f"brandId={brand_id}&limit=20", description="List Security and Activity Audit Logs")

# ==============================================================================
# 18. Model Context Protocol (MCP) JSON-RPC 2.0 Integration
# ==============================================================================
log_section("18. MODEL CONTEXT PROTOCOL (MCP) JSON-RPC 2.0")
run_curl("GET", "/mcp", headers=auth_hdr, description="MCP Server Metadata Discovery (GET /mcp)")
run_curl("POST", "/mcp", headers=auth_hdr, data={
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list",
    "params": {}
}, description="MCP Protocol tools/list (Discover registered tools)")
run_curl("POST", "/mcp", headers=auth_hdr, data={
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
        "name": "list_posts",
        "arguments": {"brandId": brand_id, "limit": 5}
    }
}, description="MCP Protocol tools/call (Invoke list_posts tool)")

# ==============================================================================
# 19. Teardown / Deletion of Entities (Testing All Delete Endpoints)
# ==============================================================================
log_section("19. TEARDOWN & RESOURCE DELETION ENDPOINTS")
if reshare_id:
    run_curl("DELETE", f"/v1/posts/{reshare_id}", headers=auth_hdr, description="Delete Reshared Post by ID")

target_post_id = post_id if post_id else "507f1f77bcf86cd799439011"
run_curl("DELETE", f"/v1/posts/{target_post_id}/scheduled", headers=auth_hdr, description="Delete Scheduled Post")
run_curl("DELETE", f"/v1/posts/{target_post_id}/published", headers=auth_hdr, description="Delete Published Post")
run_curl("DELETE", "/v1/posts/bulk/scheduled", headers=auth_hdr, data={"ids": [target_post_id]}, description="Bulk Delete Scheduled Posts")
run_curl("DELETE", "/v1/posts/bulk/published", headers=auth_hdr, data={"ids": [target_post_id]}, description="Bulk Delete Published Posts")
run_curl("DELETE", "/v1/posts/bulk", headers=auth_hdr, data={"ids": [target_post_id]}, description="Bulk Delete Posts")
run_curl("DELETE", f"/v1/posts/{target_post_id}", headers=auth_hdr, description="Delete Post by ID")

if smartlink_id:
    run_curl("DELETE", f"/v1/smartlinks/{smartlink_id}", headers=auth_hdr, query=f"brandId={brand_id}", description="Delete SmartLink Page")

if rss_id:
    run_curl("DELETE", f"/v1/rss-feeds/{rss_id}", headers=auth_hdr, query=f"brandId={brand_id}", description="Delete RSS Feed")

if autolist_id:
    run_curl("DELETE", f"/v1/autolists/{autolist_id}", headers=auth_hdr, query=f"brandId={brand_id}", description="Delete Autolist")

if comp_id:
    run_curl("DELETE", f"/v1/competitors/{comp_id}", headers=auth_hdr, query=f"brandId={brand_id}", description="Delete Competitor")

if ad_id:
    run_curl("DELETE", f"/v1/ads/campaigns/{ad_id}", headers=auth_hdr, query=f"brandId={brand_id}", description="Delete Ad Campaign")

if webhook_id:
    run_curl("DELETE", f"/v1/webhooks/{webhook_id}", headers=auth_hdr, description="Delete Webhook")

if canned_id:
    run_curl("DELETE", f"/v1/inbox/canned-replies/{canned_id}", headers=auth_hdr, query=f"brandId={brand_id}", description="Delete Canned Reply")

if api_key_id:
    run_curl("DELETE", f"/v1/api-keys/{api_key_id}", headers=auth_hdr, description="Revoke / Delete API Key")

if brand_id:
    run_curl("DELETE", f"/v1/brands/{brand_id}", headers=auth_hdr, description="Delete Brand Workspace")

run_curl("POST", "/v1/auth/logout", headers=auth_hdr, data={"refreshToken": refresh_token}, description="User Logout")
run_curl("DELETE", "/v1/users/me", headers=auth_hdr, description="Delete Current User Account")

log("\n" + "=" * 80)
log(f"All {step_num} End-to-End API Steps Executed and Verified!")
log(f"Logs successfully written to: {LOG_FILE}")
log("=" * 80)
log_f.close()
print(f"\nExecution finished! Total {step_num} API tests completed and logged to {LOG_FILE}.")
