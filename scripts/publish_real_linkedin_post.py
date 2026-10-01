import urllib.request, json, time
from pymongo import MongoClient
from bson import ObjectId

client = MongoClient('mongodb://localhost:27017')
user = client.xsocial.users.find_one({'email': 'developer@xsocial.com'})
handle = client.xsocial.socialhandles.find_one({'platform': 'LINKEDIN'})

# 1. Login
login_payload = {
    'email': 'developer@xsocial.com',
    'password': 'StaticPassword123!'
}
req = urllib.request.Request('http://localhost:5000/v1/auth/login', 
                             data=json.dumps(login_payload).encode('utf-8'), 
                             headers={'Content-Type': 'application/json'})
with urllib.request.urlopen(req) as resp:
    login_data = json.loads(resp.read().decode())
    token = login_data.get('token') or login_data.get('data', {}).get('tokens', {}).get('accessToken')

auth_hdr = {'Authorization': f'Bearer {token}', 'Content-Type': 'application/json'}

# 2. Find or Create Brand
brand = client.xsocial.brands.find_one({'userId': user['_id']})
if not brand:
    brand_req = urllib.request.Request('http://localhost:5000/v1/brands', 
                                       data=json.dumps({'name': 'XSocial Core Workspace', 'timezone': 'UTC'}).encode('utf-8'), 
                                       headers=auth_hdr)
    with urllib.request.urlopen(brand_req) as b_resp:
        brand_data = json.loads(b_resp.read().decode())
        brand_id = brand_data['id']
else:
    brand_id = str(brand['_id'])

# Associate LinkedIn handle with brand
client.xsocial.socialhandles.update_one({'_id': handle['_id']}, {'$set': {'brandId': ObjectId(brand_id)}})

# 3. Create Post with First Comment
caption_text = (
    "Excited to share our latest architecture milestone with the engineering community 🚀\n\n"
    "We have engineered a unified, enterprise-grade social orchestration engine supporting real-time publishing, "
    "approval workflows, link-in-bio analytics, and multi-network syndication.\n\n"
    "Key engineering highlights:\n"
    "• High-throughput asynchronous message dispatching\n"
    "• Automated first comment sequencing\n"
    "• Model Context Protocol (MCP) tool integration\n"
    "• Granular RBAC brand workspace isolation\n\n"
    "What are the biggest challenges your team faces when orchestrating multi-channel distribution pipelines? "
    "#engineering #softwaredevelopment #techinnovation #saas"
)

first_comment_text = (
    "For developers interested in the architecture patterns and MCP integrations, "
    "check out the documentation and release notes: https://github.com #developer #architecture #cloud"
)

post_payload = {
    'brandId': brand_id,
    'title': 'Next-Gen Multi-Platform Architecture',
    'caption': caption_text,
    'firstComment': first_comment_text,
    'selectedPlatforms': ['linkedin'],
    'selectedAccountIds': [str(handle['_id'])],
    'postType': 'text',
    'status': 'draft'
}

req = urllib.request.Request('http://localhost:5000/v1/posts', 
                             data=json.dumps(post_payload).encode('utf-8'), 
                             headers=auth_hdr)
with urllib.request.urlopen(req) as resp:
    post_data = json.loads(resp.read().decode())
    post_id = post_data['id']

print(f"Post created in draft: {post_id}")

# 4. Publish live to LinkedIn
pub_req = urllib.request.Request(f'http://localhost:5000/v1/posts/{post_id}/publish', 
                                 data=json.dumps({}).encode('utf-8'), 
                                 headers=auth_hdr)
with urllib.request.urlopen(pub_req) as resp:
    pub_result = json.loads(resp.read().decode())
    print("\n=== PUBLISH RESPONSE ===")
    print(json.dumps(pub_result, indent=2))
