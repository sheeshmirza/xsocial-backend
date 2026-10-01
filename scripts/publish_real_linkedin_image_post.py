import urllib.request, json, time
from pymongo import MongoClient
from bson import ObjectId

client = MongoClient('mongodb://localhost:27017')
user = client.xsocial.users.find_one({'email': 'developer@xsocial.com'})
handle = client.xsocial.socialhandles.find_one({'platform': 'LINKEDIN'})

# 1. Login as static user
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

# 3. Create Image Post with First Comment
ts = int(time.time())
caption_text = (
    "Designing resilient, distributed systems demands continuous innovation and disciplined engineering ⚡\n\n"
    "Modern cloud-native platforms rely on event-driven decoupling, observable microservices, "
    "and automated workflows to deliver 99.99% availability at scale.\n\n"
    "Key pillars for scaling infrastructure:\n"
    "1️⃣ Asynchronous decoupled message streaming\n"
    "2️⃣ Resilient circuit breakers & backoff retry queues\n"
    "3️⃣ Continuous telemetry, tracing & automated incident recovery\n\n"
    "What are your go-to architectural principles when engineering high-scale distributed systems? "
    "#cloudcomputing #softwarearchitecture #devops #systemdesign #engineering"
)

first_comment_text = (
    "Check out the detailed technical breakdown and architecture benchmarks: https://github.com "
    "#systemdesign #cloud #opensource #infrastructure"
)

image_url = "https://images.unsplash.com/photo-1518770660439-4636190af475"

post_payload = {
    'brandId': brand_id,
    'title': 'Building Scalable Cloud Infrastructure & Modern Architecture',
    'caption': caption_text,
    'firstComment': first_comment_text,
    'selectedPlatforms': ['linkedin'],
    'selectedAccountIds': [str(handle['_id'])],
    'postType': 'image',
    'media': [{'type': 'image', 'url': image_url}],
    'status': 'draft'
}

req = urllib.request.Request('http://localhost:5000/v1/posts', 
                             data=json.dumps(post_payload).encode('utf-8'), 
                             headers=auth_hdr)
with urllib.request.urlopen(req) as resp:
    post_data = json.loads(resp.read().decode())
    post_id = post_data['id']

print(f"Image post created in draft: {post_id}")

# 4. Publish live to LinkedIn
pub_req = urllib.request.Request(f'http://localhost:5000/v1/posts/{post_id}/publish', 
                                 data=json.dumps({}).encode('utf-8'), 
                                 headers=auth_hdr)
with urllib.request.urlopen(pub_req) as resp:
    pub_result = json.loads(resp.read().decode())
    print("\n=== PUBLISH RESPONSE ===")
    print(json.dumps(pub_result, indent=2))
