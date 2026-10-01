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

# 3. Create Multi-Image Carousel Post with First Comment
ts = int(time.time())
caption_text = (
    "Swipe through our 3-part framework for architecting resilient cloud systems in 2026 📊\n\n"
    "Slide 1: Real-time Telemetry & Data Observability\n"
    "Slide 2: Decoupled Event-Driven Microservices\n"
    "Slide 3: High-Availability Edge & Serverless Compute\n\n"
    "Which layer in your production infrastructure requires the most attention this quarter? "
    "#cloudarchitecture #systemdesign #devops #bigdata #engineering"
)

first_comment_text = (
    "Check out the comprehensive system blueprints, open-source repos, and architecture slide decks here: "
    "https://github.com #systemdesign #cloud #opensource #architecture"
)

carousel_media = [
    {
        'type': 'image',
        'url': 'https://images.unsplash.com/photo-1551288049-bebda4e38f71' # Analytics Dashboard
    },
    {
        'type': 'image',
        'url': 'https://images.unsplash.com/photo-1504384308090-c894fdcc538d' # Technology Architecture
    },
    {
        'type': 'image',
        'url': 'https://images.unsplash.com/photo-1558494949-ef010cbdcc31' # Cloud Infrastructure
    }
]

post_payload = {
    'brandId': brand_id,
    'title': '3-Part Framework for Resilient Cloud Systems',
    'caption': caption_text,
    'firstComment': first_comment_text,
    'selectedPlatforms': ['linkedin'],
    'selectedAccountIds': [str(handle['_id'])],
    'postType': 'carousel',
    'media': carousel_media,
    'status': 'draft'
}

req = urllib.request.Request('http://localhost:5000/v1/posts', 
                             data=json.dumps(post_payload).encode('utf-8'), 
                             headers=auth_hdr)
with urllib.request.urlopen(req) as resp:
    post_data = json.loads(resp.read().decode())
    post_id = post_data['id']

print(f"Carousel post created in draft: {post_id}")

# 4. Publish live to LinkedIn
pub_req = urllib.request.Request(f'http://localhost:5000/v1/posts/{post_id}/publish', 
                                 data=json.dumps({}).encode('utf-8'), 
                                 headers=auth_hdr)
with urllib.request.urlopen(pub_req) as resp:
    pub_result = json.loads(resp.read().decode())
    print("\n=== PUBLISH RESPONSE ===")
    print(json.dumps(pub_result, indent=2))
