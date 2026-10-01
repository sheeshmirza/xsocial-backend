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

# 3. Create Post with Poll and First Comment
ts = int(time.time())
caption_text = (
    "📊 Quick tech pulse check for software architects and engineering leaders!\n\n"
    "As systems scale across cloud-native environments, orchestration complexity often shifts between architectural paradigms.\n\n"
    "Which backend paradigm is delivering the highest velocity and reliability for your production services in 2026? "
    "#tech #architecture #golang #cloud #engineering"
)

first_comment_text = (
    "Drop your detailed thoughts in the thread below! For teams building with Go and reactive microservices, "
    "we will be sharing benchmark findings in our upcoming whitepaper."
)

post_payload = {
    'brandId': brand_id,
    'title': 'Production Backend Paradigm Poll',
    'caption': caption_text,
    'selectedPlatforms': ['linkedin'],
    'selectedAccountIds': [str(handle['_id'])],
    'postType': 'poll',
    'poll': {
        'question': 'Which backend paradigm dominates your 2026 production stack?',
        'options': [
            {'text': 'Golang Microservices'},
            {'text': 'Event-Driven Serverless'},
            {'text': 'Modular Monoliths'},
            {'text': 'Rust High-Perf Core'}
        ],
        'settings': {
            'duration': 'SEVEN_DAYS',
            'voteSelectionType': 'SINGLE_VOTE'
        }
    },
    'firstComment': first_comment_text,
    'status': 'draft'
}

req = urllib.request.Request('http://localhost:5000/v1/posts', 
                             data=json.dumps(post_payload).encode('utf-8'), 
                             headers=auth_hdr)
with urllib.request.urlopen(req) as resp:
    post_data = json.loads(resp.read().decode())
    post_id = post_data['id']
print(f"Created LinkedIn Poll draft post: {post_id}")

# 4. Publish Post to Live LinkedIn Feed
pub_req = urllib.request.Request(f'http://localhost:5000/v1/posts/{post_id}/publish', 
                                 data=json.dumps({}).encode('utf-8'), 
                                 headers=auth_hdr)
with urllib.request.urlopen(pub_req) as resp:
    pub_data = json.loads(resp.read().decode())
    print(f"Publish HTTP Status: {resp.status}")
    print("Platform Responses:")
    print(json.dumps(pub_data.get('post', {}).get('platformResponses'), indent=2))
