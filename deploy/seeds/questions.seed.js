// Run against DB "peerprep" and upsert a few sample docs.

// For re-seeding use this
// docker compose -f deploy/docker-compose.yaml up -d mongo && sleep 3 && docker compose -f deploy/docker-compose.yaml exec mongo mongosh peerprep --eval "db.questions.deleteMany({})" && docker compose -f deploy/docker-compose.yaml run --rm mongo-seed

const conn = new Mongo("mongodb://mongo:27017");
const db = conn.getDB("peerprep");
const col = db.getCollection("questions");

const now = new Date();
const docs = [
  {
    id: 1,
    title: "Two Sum (Leetcode version)",
    difficulty: "Easy",
    topic_tags: ["Array", "Hash Table"],
    prompt_markdown:
      "Given an array of integers nums and an integer target, return indices of the two numbers such that they add up to target.",
    constraints: "2 <= nums.length <= 10^4",
    test_cases: [
      {
        input: "nums = [2,7,11,15], target = 9",
        output: "[0,1]",
        description: "nums[0] + nums[1] == 9",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 2,
    title: "Valid Anagram",
    difficulty: "Easy",
    topic_tags: ["Hash Table", "String"],
    prompt_markdown:
      "Given two strings s and t, return true if t is an anagram of s.",
    constraints: "1 <= s.length, t.length <= 5 * 10^4",
    test_cases: [
      {
        input: 's = "anagram", t = "nagaram"',
        output: "true",
        description: "Both strings contain same characters",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 3,
    title: "LRU Cache",
    difficulty: "Medium",
    topic_tags: ["Design", "Hash Table"],
    prompt_markdown:
      "Design a data structure that follows the constraints of a Least Recently Used (LRU) cache.",
    constraints: "1 <= capacity <= 3000",
    test_cases: [
      {
        input: "capacity = 2",
        output: "null",
        description: "LRU cache with capacity 2",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 4,
    title: "Merge Two Sorted Lists",
    difficulty: "Easy",
    topic_tags: ["Linked List"],
    prompt_markdown:
      "You are given the heads of two sorted linked lists list1 and list2.",
    constraints: "The number of nodes in both lists is in the range [0, 50]",
    test_cases: [
      {
        input: "list1 = [1,2,4], list2 = [1,3,4]",
        output: "[1,1,2,3,4,4]",
        description: "Merged list",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 5,
    title: "Longest Substring Without Repeating Characters",
    difficulty: "Medium",
    topic_tags: ["Hash Table", "String", "Sliding Window"],
    prompt_markdown:
      "Given a string s, find the length of the longest substring without repeating characters.",
    constraints: "0 <= s.length <= 5 * 10^4",
    test_cases: [
      { input: 's = "abcabcbb"', output: "3", description: 'Answer is "abc"' },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 6,
    title: "Reverse Linked List",
    difficulty: "Easy",
    topic_tags: ["Linked List"],
    prompt_markdown:
      "Given the head of a singly linked list, reverse the list.",
    constraints: "The number of nodes in the list is the range [0, 5000]",
    test_cases: [
      {
        input: "head = [1,2,3,4,5]",
        output: "[5,4,3,2,1]",
        description: "Reversed list",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 7,
    title: "Container With Most Water",
    difficulty: "Medium",
    topic_tags: ["Array", "Two Pointers"],
    prompt_markdown: "You are given an integer array height of length n.",
    constraints: "2 <= n <= 10^5",
    test_cases: [
      {
        input: "height = [1,8,6,2,5,4,8,3,7]",
        output: "49",
        description: "Max area",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 8,
    title: "Climbing Stairs",
    difficulty: "Easy",
    topic_tags: ["Dynamic Programming"],
    prompt_markdown:
      "You are climbing a staircase. It takes n steps to reach the top.",
    constraints: "1 <= n <= 45",
    test_cases: [
      { input: "n = 3", output: "3", description: "Three ways to climb" },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 9,
    title: "Binary Tree Level Order Traversal",
    difficulty: "Medium",
    topic_tags: ["Tree", "Breadth-First Search"],
    prompt_markdown:
      "Given the root of a binary tree, return the level order traversal.",
    constraints: "The number of nodes in the tree is in the range [0, 2000]",
    test_cases: [
      {
        input: "root = [3,9,20,null,null,15,7]",
        output: "[[3],[9,20],[15,7]]",
        description: "Level order",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 10,
    title: "Maximum Subarray",
    difficulty: "Medium",
    topic_tags: ["Array", "Dynamic Programming"],
    prompt_markdown:
      "Given an integer array nums, find the contiguous subarray with the largest sum.",
    constraints: "1 <= nums.length <= 10^5",
    test_cases: [
      {
        input: "nums = [-2,1,-3,4,-1,2,1,-5,4]",
        output: "6",
        description: "Max sum is 6",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 11,
    title: "Best Time to Buy and Sell Stock",
    difficulty: "Easy",
    topic_tags: ["Array", "Dynamic Programming"],
    prompt_markdown:
      "You are given an array prices where prices[i] is the price of stock on ith day.",
    constraints: "1 <= prices.length <= 10^5",
    test_cases: [
      {
        input: "prices = [7,1,5,3,6,4]",
        output: "5",
        description: "Buy low sell high",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 12,
    title: "Valid Parentheses",
    difficulty: "Easy",
    topic_tags: ["Stack", "String"],
    prompt_markdown:
      "Given a string s containing just the characters '(', ')', '{', '}', '[' and ']'.",
    constraints: "1 <= s.length <= 10^4",
    test_cases: [
      { input: 's = "()[]{}"', output: "true", description: "Valid brackets" },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 13,
    title: "Kth Largest Element in an Array",
    difficulty: "Medium",
    topic_tags: ["Heap", "Sorting"],
    prompt_markdown:
      "Given an integer array nums and an integer k, return the kth largest element.",
    constraints: "1 <= k <= nums.length <= 10^5",
    test_cases: [
      {
        input: "nums = [3,2,1,5,6,4], k = 2",
        output: "5",
        description: "2nd largest is 5",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 14,
    title: "Coin Change",
    difficulty: "Medium",
    topic_tags: ["Dynamic Programming"],
    prompt_markdown:
      "You are given coins of different denominations and a total amount of money.",
    constraints: "1 <= coins.length <= 12",
    test_cases: [
      {
        input: "coins = [1,3,4], amount = 6",
        output: "2",
        description: "Min 2 coins",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
  {
    id: 15,
    title: "Word Break",
    difficulty: "Medium",
    topic_tags: ["Dynamic Programming", "String"],
    prompt_markdown: "Given a string s and a dictionary of strings wordDict.",
    constraints: "1 <= s.length <= 300",
    test_cases: [
      {
        input: 's = "leetcode", wordDict = ["leet","code"]',
        output: "true",
        description: "Can be segmented",
      },
    ],
    image_urls: [],
    status: "active",
    author: "leetcode",
    created_at: now,
    updated_at: now,
  },
];

// ensure collection exists
db.createCollection("questions");

// upsert by Title (Unique key for Slugs)
for (const d of docs) {
  col.updateOne({ title: d.title }, { $setOnInsert: d }, { upsert: true });
}

print(`Seed complete. questions count = ${col.countDocuments({})}`);
