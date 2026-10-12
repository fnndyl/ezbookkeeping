import json, urllib.request, urllib.error, uuid, time
from concurrent.futures import ThreadPoolExecutor
BASE='http://127.0.0.1:18086/api/'
headers={'Content-Type':'application/json','X-Timezone-Offset':'120','X-Timezone-Name':'Africa/Johannesburg'}
def call(path, body):
    request=urllib.request.Request(BASE+path,data=json.dumps(body).encode(),headers=headers)
    try:
        with urllib.request.urlopen(request,timeout=30) as response: data=json.load(response)
    except urllib.error.HTTPError as error:
        data=json.load(error)
    assert data.get('success'), (path, data)
    return data.get('result')
for attempt in range(30):
    try:
        urllib.request.urlopen('http://127.0.0.1:18086/',timeout=1).close();break
    except Exception: time.sleep(1)
suffix=uuid.uuid4().hex[:8]
result=call('register.json',dict(username='duplicate'+suffix,email=suffix+'@example.com',nickname='Duplicate Test',password=uuid.uuid4().hex,language='en',defaultCurrency='ZAR',categories=[]))
headers['Authorization']='Bearer '+result['token']
account=call('v1/accounts/add.json',dict(name='Checking',category=2,type=1,icon='1',iconType=0,color='000000',currency='ZAR',balance='0',balanceTime=int(time.time()),comment='',clientSessionId=uuid.uuid4().hex))
other=call('v1/accounts/add.json',dict(name='Other',category=2,type=1,icon='1',iconType=0,color='000000',currency='ZAR',balance='0',balanceTime=int(time.time()),comment='',clientSessionId=uuid.uuid4().hex))
parent=call('v1/transaction/categories/add.json',dict(name='Expenses',type=2,parentId='0',icon='1',iconType=0,color='000000',comment='',clientSessionId=uuid.uuid4().hex))
category=call('v1/transaction/categories/add.json',dict(name='Bank imports',type=2,parentId=parent['id'],icon='1',iconType=0,color='000000',comment='',clientSessionId=uuid.uuid4().hex))
qif='''!Type:Bank
D10/08/2026
T-50.00
MPURCH FOOD & CONNECT - SO 400738******8778
^
D10/06/2026
T-13.00
MPURCH Interpark Cavendish 400738******8778
^
D10/06/2026
T-3.00
M#INT PYMT FEE-127.20 STEA
^
D10/07/2026
T-350.00
MELECTRICITY 09000407750
^
D10/06/2026
T-127.20
M127.20 STEAMGAMES.C 479056*9506  04 OCT
^
D10/05/2026
T-137.97
MPURCH WOOLWORTHS          400738******8778
^
'''
boundary='test-'+uuid.uuid4().hex
body=(f'--{boundary}\r\nContent-Disposition: form-data; name="fileType"\r\n\r\nqif_mdy\r\n'
      f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="bank.qif"\r\nContent-Type: text/plain\r\n\r\n{qif}\r\n--{boundary}--\r\n').encode()
req=urllib.request.Request(BASE+'v1/transactions/parse_import.json',data=body,headers={**headers,'Content-Type':f'multipart/form-data; boundary={boundary}'})
with urllib.request.urlopen(req) as response: parsed=json.load(response)
assert parsed['success'],parsed
rows=parsed['result']['items'];assert len(rows)==6
candidates=[dict(type=row['type'],time=row['time'],utcOffset=row['utcOffset'],amount=row['sourceAmount'],comment=row['comment']) for row in rows]
def check(items=candidates): return call('v1/transactions/check_import_duplicates.json',dict(transactions=items))
assert not any(match['duplicate'] for match in check())
def transaction(row,acct=account['id']):
    return dict(type=row['type'],time=row['time'],utcOffset=row['utcOffset'],sourceAmount=row['sourceAmount'],comment=row['comment'],sourceAccountId=acct,destinationAccountId='0',destinationAmount=0,categoryId=category['id'],tagIds=[],pictureIds=[])
def save(items,ordinals=None,allow=False):
    payload=dict(transactions=items,clientSessionId=uuid.uuid4().hex,allowDuplicates=allow)
    if ordinals is not None:payload['duplicateOccurrences']=ordinals
    return call('v1/transactions/import.json',payload)
requests=[transaction(row) for row in rows]
assert save(requests)==6
assert all(match['duplicate'] for match in check())
assert save(requests)==0
assert save([transaction(row,other['id']) for row in rows])==0 # account is deliberately ignored
assert save(requests,allow=True)==6
# An overlapping file contributes only its new transaction.
new_row=rows[0].copy();new_row['comment']='New transaction in overlapping file'
overlap=requests+[transaction(new_row)]
assert save(overlap)==1
assert save(overlap)==0
# Occurrence-aware selection and a stale preview.
base=rows[0].copy();base['comment']='Separate repeated payment';item=transaction(base)
assert save([item])==1
preview=check([dict(type=base['type'],time=base['time'],utcOffset=base['utcOffset'],amount=base['sourceAmount'],comment=base['comment'])]*2)
assert [x['duplicate'] for x in preview]==[True,False]
assert save([item],[preview[1]['occurrence']])==1
assert save([item],[preview[1]['occurrence']])==0
# Imports racing in this server process cannot both insert the same occurrence.
race=rows[1].copy();race['comment']='Concurrent import test'
with ThreadPoolExecutor(max_workers=2) as pool:
    results=list(pool.map(lambda _:save([transaction(race)]),range(2)))
assert sorted(results)==[0,1],results
print('PASS: bank QIF preview, first/repeat imports, account-independent matching, override, overlapping files, occurrence subsets, stale preview, concurrent imports')
