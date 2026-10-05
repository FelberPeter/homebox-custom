#!/usr/bin/env python3
"""Integration checks against an isolated Homebox custom test deployment."""
import base64
import struct
import zlib
import concurrent.futures
import json
import os
from pathlib import Path
import secrets
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid

BASE = os.environ.get("HOMEBOX_TEST_URL", "http://127.0.0.1:7746")
NIL = str(uuid.UUID(int=0))
HERE = Path(__file__).resolve().parent
RESULTS = []

class API:
    def __init__(self, token=""):
        self.token = token
    def request(self, method, path, data=None, expected=None, raw=False, headers=None):
        h = dict(headers or {})
        if self.token:
            h["Authorization"] = self.token
        if data is not None and not isinstance(data, bytes):
            data = json.dumps(data).encode()
            h["Content-Type"] = "application/json"
        req = urllib.request.Request(BASE + "/api/v1" + path, data=data, headers=h, method=method)
        try:
            with urllib.request.urlopen(req, timeout=120) as response:
                status, body = response.status, response.read()
        except urllib.error.HTTPError as error:
            status, body = error.code, error.read()
        if expected is not None:
            assert status in ([expected] if isinstance(expected, int) else expected), (method, path, status, body[:500])
        elif status >= 400:
            raise AssertionError((method, path, status, body[:500]))
        return body if raw else (json.loads(body) if body else None)
    def get(self, path):
        return self.request("GET", path)
    def post(self, path, data, expected=None):
        return self.request("POST", path, data, expected)

def account(label):
    public = API()
    credentials = {"email": f"{label}-{uuid.uuid4().hex[:8]}@homebox.invalid", "password": secrets.token_urlsafe(24), "name": label, "token": ""}
    public.post("/users/register", credentials, 204)
    auth = public.post("/users/login", {"username": credentials["email"], "password": credentials["password"]}, 200)
    return API(auth["token"]), credentials

def record(name):
    RESULTS.append({"check": name, "passed": True})
    print("PASS", name)

def run():
    api, credentials = account("Homebox acceptance")
    private = HERE / ".test-credentials.json"
    private.write_text(json.dumps(credentials), encoding="utf-8")
    private.chmod(0o600)
    types = api.get("/entity-types")
    if not any(t["isLocation"] for t in types):
        types.append(api.post("/entity-types", {"name": "Location", "isLocation": True, "icon": "mdi-map-marker"}))
    if not any(not t["isLocation"] for t in types):
        types.append(api.post("/entity-types", {"name": "Item", "isLocation": False, "icon": "mdi-package-variant"}))
    loc_type = next(t["id"] for t in types if t["isLocation"])
    item_type = next(t["id"] for t in types if not t["isLocation"])
    def create(name, parent=None, location=True, quantity=1):
        return api.post("/entities", {"name": name, "description": "Acceptance test data", "parentId": parent, "entityTypeId": loc_type if location else item_type, "quantity": quantity, "tagIds": []}, 201)
    def operation(source, **options):
        body = {"action": "copy", "requestId": str(uuid.uuid4()), "depth": -1, "photos": True, "parentId": NIL, **options}
        preview = api.post(f"/locations/{source}/operations", {**body, "preview": True}, 200)
        result = api.post(f"/locations/{source}/operations", body, 200)
        return body, preview, result
    garage = create("Garage")
    home = create("Wohnung")
    box = create("Sortierkasten blau", garage["id"])
    numbered_request, _, numbered = operation(box["id"], action="generate", mode="number", count=12, start=1, pattern="Fach * Sortierkasten blau", digits=0)
    children = api.get("/entities/" + box["id"])["children"]
    assert len(children) == 12 and {c["name"] for c in children} == {f"Fach {n} Sortierkasten blau" for n in range(1,13)}
    for child in children:
        detail = api.get("/entities/" + child["id"])
        assert detail["parent"]["id"] == box["id"] and not detail["attachments"]
    record("12 numbered empty direct sublocations")
    operation(box["id"], action="generate", mode="number", count=2, start=20, pattern="Fach *", digits=2)
    operation(box["id"], action="generate", mode="number", count=1, start=3, pattern="Fach *", digits=2)
    assert "Fach 03" in {c["name"] for c in api.get("/entities/" + box["id"])["children"]}
    assert len(api.get("/entities/" + box["id"])["children"]) == 15
    record("start values, padding and later additions")
    grid = create("Raster", garage["id"])
    operation(grid["id"], action="generate", mode="grid", rows=3, columns=4, rowStart="A", start=1, pattern="Fach {row}{col}")
    assert {c["name"] for c in api.get("/entities/" + grid["id"])["children"]} == {f"Fach {row}{col}" for row in "ABC" for col in range(1,5)}
    record("3 by 4 grid under the correct parent")
    source = create("Regal", garage["id"])
    child = create("Fach 1", source["id"])
    grand = create("Einsatz 17", child["id"])
    deep = create("Kleinteile", grand["id"])
    tool = create("Bohrmaschine", child["id"], False, 7)
    part = create("Bohrer", tool["id"], False, 3)
    detail = api.get("/entities/" + tool["id"])
    detail.update({"entityTypeId": item_type, "parentId": child["id"], "tagIds": [], "serialNumber": "PRIVATE-SERIAL", "purchaseFrom": "Shop", "purchaseDate": "2025-01-02", "lifetimeWarranty": True, "warrantyExpires": "2030-01-01", "warrantyDetails": "Warranty notes", "notes": "Independent notes", "fields": [{"name": "Account", "type": "text", "textValue": "Individual account", "numberValue": 0, "booleanValue": False}]})
    api.request("PUT", "/entities/" + tool["id"], detail, 200)
    for depth, expected_count in [(0,1),(1,2),(2,3),(-1,4)]:
        _, preview, result = operation(source["id"], name=f"Regal copy depth {depth}", depth=depth, items=True)
        assert result["rootId"] != source["id"] and preview["locations"] == expected_count
        assert preview["items"] == (0 if depth == 0 else 2)
        assert result["locations"] == expected_count
        copied = api.get("/entities/" + result["rootId"])
        assert copied["assetId"] != api.get("/entities/" + source["id"])["assetId"]
        if depth != 0:
            copy_child = copied["children"][0]
            copy_items = api.get("/entities?" + urllib.parse.urlencode({"parentIds": copy_child["id"]}))["items"]
            copy_tool = next(i for i in copy_items if i["name"] == "Bohrmaschine")
            assert copy_tool["id"] != tool["id"] and copy_tool["quantity"] == 7
            info = api.get("/entities/" + copy_tool["id"])
            assert not info["serialNumber"] and not info["purchaseFrom"] and not info["lifetimeWarranty"]
            assert info["notes"] == "Independent notes" and info["fields"][0]["textValue"] == "Individual account"
            parts = api.get("/entities?" + urllib.parse.urlencode({"parentIds": copy_tool["id"]}))["items"]
            assert any(i["name"] == "Bohrer" and i["quantity"] == 3 for i in parts)
    record("depth 0, 1, 2 and all; nested item copies, quantities and fresh identities")
    _, preview, _ = operation(source["id"], name="Structure only", items=False)
    assert preview["items"] == 0 and preview["excluded"] == 2
    _, _, result = operation(source["id"], name="With individual data", items=True, serialNumbers=True, purchaseWarranty=True)
    individual_child = api.get("/entities/" + result["rootId"])["children"][0]
    individual_tool = next(i for i in api.get("/entities?" + urllib.parse.urlencode({"parentIds":individual_child["id"]}))["items"] if i["name"]=="Bohrmaschine")
    info=api.get("/entities/"+individual_tool["id"])
    assert info["serialNumber"]=="PRIVATE-SERIAL" and info["purchaseFrom"]=="Shop" and info["lifetimeWarranty"]
    record("item copy off and individual data switches")
    conflict_body={"action":"copy","requestId":str(uuid.uuid4()),"name":"Regal","depth":0,"parentId":garage["id"]}
    conflict=api.post(f'/locations/{source["id"]}/operations',{**conflict_body,"preview":True})
    assert conflict["conflicts"]==["Regal"]
    api.post(f'/locations/{source["id"]}/operations',conflict_body,400)
    allowed=api.post(f'/locations/{source["id"]}/operations',{**conflict_body,"allowConflicts":True},200)
    assert allowed["rootId"]!=source["id"] and api.get("/entities/"+source["id"])["name"]=="Regal"
    record("duplicate names warned, explicit continuation and no overwrites")
    _, _, moved=operation(source["id"],action="move",name="Regal",parentId=home["id"])
    assert moved["rootId"]==source["id"]
    assert api.get("/entities/"+source["id"])["parent"]["id"]==home["id"]
    assert api.get("/entities/"+tool["id"])["parent"]["id"]==child["id"]
    assert api.get("/entities/"+part["id"])["parent"]["id"]==tool["id"]
    for invalid_parent in [source["id"],deep["id"]]:
        api.post(f'/locations/{source["id"]}/operations',{"action":"move","requestId":str(uuid.uuid4()),"name":"Regal","parentId":invalid_parent,"depth":-1},400)
    record("move preserves IDs and contents; cycles rejected")
    again=api.post(f'/locations/{box["id"]}/operations',numbered_request,200)
    assert again["replayed"] and again["rootId"]==numbered["rootId"]
    api.post(f'/locations/{box["id"]}/operations',{**numbered_request,"count":13},400)
    simultaneous={"action":"generate","requestId":str(uuid.uuid4()),"mode":"number","count":2,"start":101,"pattern":"Repeat *"}
    def submit(_):return api.post(f'/locations/{box["id"]}/operations',simultaneous,200)
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        repeated=list(pool.map(submit,range(2)))
    assert sum(bool(r["replayed"]) for r in repeated)==1
    assert len([c for c in api.get("/entities/"+box["id"])["children"] if c["name"].startswith("Repeat ")])==2
    record("retries, changed request rejection and simultaneous double submission")
    for bad in [{"count":201,"pattern":"Fach *"},{"count":12,"pattern":"{execute(code)}"},{"count":-1,"pattern":"Fach *"}]:
        api.post(f'/locations/{box["id"]}/operations',{"action":"generate","requestId":str(uuid.uuid4()),"mode":"number","start":1,**bad},400)
    outsider,_=account("Isolated tenant")
    outsider.post(f'/locations/{box["id"]}/operations',{"action":"generate","requestId":str(uuid.uuid4()),"mode":"number","count":1,"start":1,"pattern":"Fach *"},400)
    API().post(f'/locations/{box["id"]}/operations',numbered_request,[401,403])
    record("server validation, authentication and group isolation")
    # Multipart uploads exercise the actual storage and mobile upload endpoints.
    def chunk(kind,data):
        return struct.pack(">I",len(data))+kind+data+struct.pack(">I",zlib.crc32(kind+data)&0xffffffff)
    png=b"\x89PNG\r\n\x1a\n"+chunk(b"IHDR",struct.pack(">IIBBBBB",1,1,8,6,0,0,0))+chunk(b"IDAT",zlib.compress(b"\x00\x30\xa0\x50\xff"))+chunk(b"IEND",b"")
    def upload(owner,name,content,kind,mime):
        boundary="homebox-"+uuid.uuid4().hex
        fields={"name":name,"type":kind,"primary":"true" if kind=="photo" else "false"}
        chunks=[]
        for key,value in fields.items():chunks.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{key}"\r\n\r\n{value}\r\n'.encode())
        chunks.append(f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{name}"\r\nContent-Type: {mime}\r\n\r\n'.encode()+content+b"\r\n")
        chunks.append(f'--{boundary}--\r\n'.encode())
        return api.request("POST",f"/entities/{owner}/attachments",b"".join(chunks),201,headers={"Content-Type":f"multipart/form-data; boundary={boundary}"})
    media=create("Foto-Test",garage["id"])
    upload(media["id"],"photo.png",png,"photo","image/png")
    upload(media["id"],"receipt.txt",b"Receipt attachment test","receipt","text/plain")
    _,_,photo_only=operation(media["id"],name="Photos only",depth=0,photos=True,attachments=False)
    assert {a["type"] for a in api.get("/entities/"+photo_only["rootId"])["attachments"]}=={"photo"}
    _,_,all_media=operation(media["id"],name="Independent media",depth=0,photos=True,attachments=True)
    independent=api.get("/entities/"+all_media["rootId"])
    assert len(independent["attachments"])==2
    original=api.get("/entities/"+media["id"])
    assert {a["path"] for a in original["attachments"]}.isdisjoint({a["path"] for a in independent["attachments"]})
    api.request("DELETE","/entities/"+media["id"],expected=204)
    for a in independent["attachments"]:
        data=api.request("GET",f'/entities/{independent["id"]}/attachments/{a["id"]}',expected=200,raw=True)
        assert data== (png if a["type"]=="photo" else b"Receipt attachment test")
        if a.get("thumbnail"):
            assert api.request("GET",f'/entities/{independent["id"]}/attachments/{a["thumbnail"]["id"]}',expected=200,raw=True)
    # Deleting the other copy must not affect the retained independent copy.
    api.request("DELETE","/entities/"+photo_only["rootId"],expected=204)
    assert api.request("GET",f'/entities/{independent["id"]}/attachments/{independent["attachments"][0]["id"]}',expected=200,raw=True)
    record("photo and attachment options; independent files and thumbnails after deletion")
    state={"garage":garage["id"],"home":home["id"],"box":box["id"],"grid":grid["id"],"source":source["id"],"independentMedia":independent["id"],"numberedRequest":numbered_request,"numberedResult":numbered}
    (HERE/".test-state.json").write_text(json.dumps(state),encoding="utf-8")
    (HERE/"acceptance-runtime.json").write_text(json.dumps(RESULTS,indent=2),encoding="utf-8")
    print("Integration checks completed; test credentials are stored privately.")

def verify_persistence():
    credentials=json.loads((HERE/".test-credentials.json").read_text())
    token=API().post("/users/login",{"username":credentials["email"],"password":credentials["password"]})["token"]
    api=API(token)
    state=json.loads((HERE/".test-state.json").read_text())
    assert len(api.get("/entities/"+state["box"])["children"])==17
    replay=api.post(f'/locations/{state["box"]}/operations',state["numberedRequest"])
    assert replay["replayed"] and replay["rootId"]==state["numberedResult"]["rootId"]
    media=api.get("/entities/"+state["independentMedia"])
    for attachment in media["attachments"]:
        assert api.request("GET",f'/entities/{media["id"]}/attachments/{attachment["id"]}',raw=True,expected=200)
    RESULTS.extend(json.loads((HERE/"acceptance-runtime.json").read_text()))
    record("container restart preserves test records, uploads and idempotency receipts")
    (HERE/"acceptance-runtime.json").write_text(json.dumps(RESULTS,indent=2))

if __name__=="__main__":
    verify_persistence() if "--persistence" in sys.argv else run()
