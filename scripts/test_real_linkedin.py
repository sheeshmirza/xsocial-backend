import urllib.request, json, urllib.parse
from pymongo import MongoClient
from bson import ObjectId

client = MongoClient('mongodb://localhost:27017')
user = client.xsocial.users.find_one({'email': 'developer@xsocial.com'})
handle = client.xsocial.socialhandles.find_one({'platform': 'LINKEDIN'})

# 1. Login
req = urllib.request.Request('http://localhost:5000/v1/auth/login', data=json.dumps({
    'email': 'developer@xsocial.com',
    'password': 'StaticPassword123!'
}).encode(), headers={'Content-Type': 'application/json'})

with urllib.request.urlopen(req) as resp:
    login_data = json.loads(resp.read().decode())
    token = login_data.get('token') or login_data.get('data', {}).get('tokens', {}).get('accessToken')

auth_hdr = {'Authorization': f'Bearer {token}', 'Content-Type': 'application/json'}

# 2. Create brand
req = urllib.request.Request('http://localhost:5000/v1/brands', data=json.dumps({
    'name': 'LinkedIn Real Testing Brand 2',
    'timezone': 'UTC'
}).encode(), headers=auth_hdr)

with urllib.request.urlopen(req) as resp:
    brand = json.loads(resp.read().decode())
    brand_id = brand['id']

# 3. Associate LinkedIn handle with brand
client.xsocial.socialhandles.update_one({'_id': handle['_id']}, {'$set': {'brandId': ObjectId(brand_id)}})

# 4. Create Post with First Comment
post_payload = {
    'brandId': brand_id,
    'title': 'Real LinkedIn First Comment Test',
    'caption': 'Testing automated real LinkedIn post with first comment! #tech #api',
    'firstComment': 'Automated first comment from XSocial testing suite! #firstcomment',
    'selectedPlatforms': ['linkedin'],
    'selectedAccountIds': [str(handle['_id'])],
    'postType': 'text',
    'status': 'draft'
}

req = urllib.request.Request('http://localhost:5000/v1/posts', data=json.dumps(post_payload).encode(), headers=auth_hdr)
with urllib.request.urlopen(req) as resp:
    post = json.loads(resp.read().decode())
    post_id = post['id']
print('Created post with first comment:', post_id)

# 5. Publish live to LinkedIn
req = urllib.request.Request(f'http://localhost:5000/v1/posts/{post_id}/publish', data=json.dumps({}).encode(), headers=auth_hdr)
with urllib.request.urlopen(req) as resp:
    print('Publish status:', resp.status)
    pub_data = json.loads(resp.read().decode())
    print('Publish result:', json.dumps(pub_data.get('post', {}).get('platformResponses'), indent=2))

# 6. Delete published post via API (triggers remote cleanup on LinkedIn)
req = urllib.request.Request(f'http://localhost:5000/v1/posts/{post_id}/published', headers=auth_hdr, method='DELETE')
with urllib.request.urlopen(req) as resp:
    print('Delete published status via API (remote LinkedIn delete):', resp.status)

# 7. Delete test brand
del_req = urllib.request.Request(f'http://localhost:5000/v1/brands/{brand_id}', headers=auth_hdr, method='DELETE')
with urllib.request.urlopen(del_req) as resp:
    print('Deleted test brand workspace:', resp.status)
