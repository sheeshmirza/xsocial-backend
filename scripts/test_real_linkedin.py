import urllib.request, json, urllib.parse, time
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
    'name': 'LinkedIn Real Testing Brand Multi',
    'timezone': 'UTC'
}).encode(), headers=auth_hdr)

with urllib.request.urlopen(req) as resp:
    brand = json.loads(resp.read().decode())
    brand_id = brand['id']

# 3. Associate LinkedIn handle with brand
client.xsocial.socialhandles.update_one({'_id': handle['_id']}, {'$set': {'brandId': ObjectId(brand_id)}})

# Test 1: Image Post
ts = int(time.time())
image_payload = {
    'brandId': brand_id,
    'title': 'Real LinkedIn Image Post Test',
    'caption': f'Testing automated real LinkedIn post with real image asset [{ts}]! #tech #media',
    'selectedPlatforms': ['linkedin'],
    'selectedAccountIds': [str(handle['_id'])],
    'postType': 'image',
    'media': [{'type': 'image', 'url': 'https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe'}],
    'status': 'draft'
}

req = urllib.request.Request('http://localhost:5000/v1/posts', data=json.dumps(image_payload).encode(), headers=auth_hdr)
with urllib.request.urlopen(req) as resp:
    img_post = json.loads(resp.read().decode())
    img_post_id = img_post['id']
print('Created image post in draft:', img_post_id)

req = urllib.request.Request(f'http://localhost:5000/v1/posts/{img_post_id}/publish', data=json.dumps({}).encode(), headers=auth_hdr)
with urllib.request.urlopen(req) as resp:
    pub_img = json.loads(resp.read().decode())
    print('Image publish status:', resp.status)
    print('Image platform responses:', json.dumps(pub_img.get('post', {}).get('platformResponses'), indent=2))

# Delete image post via /published
req = urllib.request.Request(f'http://localhost:5000/v1/posts/{img_post_id}/published', headers=auth_hdr, method='DELETE')
with urllib.request.urlopen(req) as resp:
    print('Delete published image post status (remote LinkedIn clean):', resp.status)

# Delete brand workspace
del_req = urllib.request.Request(f'http://localhost:5000/v1/brands/{brand_id}', headers=auth_hdr, method='DELETE')
with urllib.request.urlopen(del_req) as resp:
    print('Deleted test brand workspace:', resp.status)
