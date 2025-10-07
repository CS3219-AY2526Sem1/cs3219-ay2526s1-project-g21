// Run against DB "peerprep" and upsert a few sample docs.

const conn = new Mongo("mongodb://mongo:27017");
const db = conn.getDB("peerprep");
const col = db.getCollection("questions");

const now = new Date();
const docs = [
  {
    id: "1",
    title: "Two Sum",
    difficulty: "Easy",
    tags: ["Array", "Hash Table"],
    status: "active",
    createdAt: now,
    updatedAt: now
  },
  {
    id: "2",
    title: "Valid Anagram",
    difficulty: "Easy",
    tags: ["Hash Table", "String"],
    status: "active",
    createdAt: now,
    updatedAt: now
  },
  {
    id: "3",
    title: "LRU Cache",
    difficulty: "Medium",
    tags: ["Design", "Hash Table"],
    status: "active",
    createdAt: now,
    updatedAt: now
  }
];

// ensure collection exists
db.createCollection("questions");

// upsert by Title (Unique key for Slugs)
for (const d of docs) {
  col.updateOne({ title: d.title }, { $setOnInsert: d }, { upsert: true });
}

print(`Seed complete. questions count = ${col.countDocuments({})}`);
