import assert from "node:assert/strict";
import test from "node:test";
import { loadFamily, saveList, type ListInput } from "./api.ts";

const list: ListInput = {
  id: "11111111111111111111111111111111",
  kind: "personal",
  name: "Robin",
  revision: 1,
  wishes: [{ id: "22222222222222222222222222222222", title: "Watercolor set" }],
};

test("saving a wish omits blank optional fields from the request", async (t) => {
  t.mock.method(globalThis, "fetch", async (path: string, request: RequestInit) => {
    assert.equal(path, "/api/wish/lists");
    assert.equal(request.method, "PUT");
    assert.equal(typeof request.body, "string");
    assert.deepEqual(JSON.parse(request.body), list);
    return Response.json({ ...list, editable: true, revision: 2 });
  });

  const result = await saveList({ ...list, wishes: [{ ...list.wishes[0], url: "", notes: "" }] });
  assert.equal(result.kind, "ok");
});

test("a missing or unusable link leaves the wish available to read", async (t) => {
  t.mock.method(globalThis, "fetch", async () => Response.json({
    lists: [
      { ...list, editable: true, wishes: [{ ...list.wishes[0], url: "", notes: "A small one" }] },
    ],
    can_manage: false,
  }));

  const result = await loadFamily();
  assert.equal(result.kind, "ok");
  assert.equal(result.value.lists[0].wishes[0].title, "Watercolor set");
  assert.equal(result.value.lists[0].wishes[0].notes, "A small one");
  assert.equal(result.value.lists[0].wishes[0].url, undefined);
});
