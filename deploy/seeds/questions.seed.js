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
  },
  {
    id: "4",
    title: "Merge Two Sorted Lists",
    difficulty: "Easy",
    tags: ["Linked List"],
    status: "active",
    createdAt: now,
    updatedAt: now
  },
  {
    id: "5",
    title: "Longest Substring Without Repeating Characters",
    difficulty: "Medium",
    tags: ["Hash Table", "String", "Sliding Window"],
    status: "active",
    createdAt: now,
    updatedAt: now
  },
  {
    id: "6",
    title: "Reverse Linked List",
    difficulty: "Easy",
    tags: ["Linked List"],
    status: "active",
    createdAt: now,
    updatedAt: now
  },
  {
    id: "7",
    title: "Container With Most Water",
    difficulty: "Medium",
    tags: ["Array", "Two Pointers"],
    status: "active",
    createdAt: now,
    updatedAt: now,
  },
  {
    id: "8",
    title: "Climbing Stairs",
    difficulty: "Easy",
    tags: ["Dynamic Programming"],
    status: "active",
    createdAt: now,
    updatedAt: now,
  },
  {
    id: "9",
    title: "Binary Tree Level Order Traversal",
    difficulty: "Medium",
    tags: ["Tree", "Breadth-First Search"],
    status: "active",
    createdAt: now,
    updatedAt: now,
  },
  {
    id: "10",
    title: "Maximum Subarray",
    difficulty: "Medium",
    tags: ["Array", "Dynamic Programming"],
    status: "active",
    createdAt: now,
    updatedAt: now,
  },
  {
    id: "11",
    title: "Best Time to Buy and Sell Stock",
    difficulty: "Easy",
    tags: ["Array", "Dynamic Programming"],
    status: "active",
    createdAt: now,
    updatedAt: now,
  },
  {
    id: "12",
    title: "Valid Parentheses",
    difficulty: "Easy",
    tags: ["Stack", "String"],
    status: "active",
    createdAt: now,
    updatedAt: now,
  },
  {
    id: "13",
    title: "Kth Largest Element in an Array",
    difficulty: "Medium",
    tags: ["Heap", "Sorting"],
    status: "active",
    createdAt: now,
    updatedAt: now,
  },
  {
    id: "14",
    title: "Coin Change",
    difficulty: "Medium",
    tags: ["Dynamic Programming"],
    status: "active",
    createdAt: now,
    updatedAt: now,
  },
  {
    id: "15",
    title: "Word Break",
    difficulty: "Medium",
    tags: ["Dynamic Programming", "String"],
    status: "active",
    createdAt: now,
    updatedAt: now,
  },
];


// ensure collection exists
db.createCollection("questions");

// upsert by Title (Unique key for Slugs)
for (const d of docs) {
  col.updateOne({ title: d.title }, { $setOnInsert: d }, { upsert: true });
}

print(`Seed complete. questions count = ${col.countDocuments({})}`);
