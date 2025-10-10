import type { Question } from "@/types/question";
import { getAllQuestions } from "@/api/questions";

const mockQuestions: Question[] = [
  {
    id: "1",
    title: "Jungwoo and Bananas",
    topic_tags: ["Greedy"],
    difficulty: "Hard",
    status: "active",
    prompt_markdown: "# Jungwoo and Bananas\n\nSolve this greedy problem...",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: "2", 
    title: "Find Duplicate Peak Element in Array",
    topic_tags: ["Binary Search"],
    difficulty: "Medium",
    status: "active",
    prompt_markdown: "# Find Duplicate Peak Element\n\nUse binary search to find...",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: "3",
    title: "House Robbers I", 
    topic_tags: ["Dynamic Programming"],
    difficulty: "Easy",
    status: "active",
    prompt_markdown: "# House Robbers\n\nYou are a professional robber...",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: "4",
    title: "Gas Stations",
    topic_tags: ["Dynamic Programming"],
    difficulty: "Medium", 
    status: "active",
    prompt_markdown: "# Gas Stations\n\nFind the optimal gas station...",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: "5",
    title: "Reverse a Linked List",
    topic_tags: ["Linked List"], 
    difficulty: "Hard",
    status: "active",
    prompt_markdown: "# Reverse Linked List\n\nReverse a singly linked list...",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: "6",
    title: "Two Sum",
    topic_tags: ["Hash Table"],
    difficulty: "Easy",
    status: "active", 
    prompt_markdown: "# Two Sum\n\nGiven an array of integers...",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  // TODO: Extra data to be used later for testing pagination... 
  // {
  //   id: "8",
  //   title: "Valid Parentheses",
  //   topic_tags: ["Stack"],
  //   difficulty: "Easy",
  //   status: "active",
  //   prompt_markdown: "# Valid Parentheses\n\nGiven a string containing just the characters...",
  //   created_at: new Date().toISOString(),
  //   updated_at: new Date().toISOString(),
  // },
  // {
  //   id: "9",
  //   title: "Merge Two Sorted Lists",
  //   topic_tags: ["Linked List"],
  //   difficulty: "Easy",
  //   status: "active",
  //   prompt_markdown: "# Merge Two Sorted Lists\n\nYou are given the heads of two sorted linked lists...",
  //   created_at: new Date().toISOString(),
  //   updated_at: new Date().toISOString(),
  // },
  // {
  //   id: "10",
  //   title: "Best Time to Buy and Sell Stock",
  //   topic_tags: ["Array"],
  //   difficulty: "Easy",
  //   status: "active",
  //   prompt_markdown: "# Best Time to Buy and Sell Stock\n\nYou are given an array prices...",
  //   created_at: new Date().toISOString(),
  //   updated_at: new Date().toISOString(),
  // },
  // {
  //   id: "11",
  //   title: "Maximum Subarray",
  //   topic_tags: ["Dynamic Programming"],
  //   difficulty: "Medium",
  //   status: "active",
  //   prompt_markdown: "# Maximum Subarray\n\nGiven an integer array nums...",
  //   created_at: new Date().toISOString(),
  //   updated_at: new Date().toISOString(),
  // },
  // {
  //   id: "12",
  //   title: "Climbing Stairs",
  //   topic_tags: ["Dynamic Programming"],
  //   difficulty: "Easy",
  //   status: "active",
  //   prompt_markdown: "# Climbing Stairs\n\nYou are climbing a staircase...",
  //   created_at: new Date().toISOString(),
  //   updated_at: new Date().toISOString(),
  // },
  // {
  //   id: "13",
  //   title: "Binary Tree Inorder Traversal",
  //   topic_tags: ["Tree"],
  //   difficulty: "Easy",
  //   status: "active",
  //   prompt_markdown: "# Binary Tree Inorder Traversal\n\nGiven the root of a binary tree...",
  //   created_at: new Date().toISOString(),
  //   updated_at: new Date().toISOString(),
  // },
  // {
  //   id: "14",
  //   title: "Symmetric Tree",
  //   topic_tags: ["Tree"],
  //   difficulty: "Easy",
  //   status: "active",
  //   prompt_markdown: "# Symmetric Tree\n\nGiven the root of a binary tree...",
  //   created_at: new Date().toISOString(),
  //   updated_at: new Date().toISOString(),
  // },
  // {
  //   id: "15",
  //   title: "Maximum Depth of Binary Tree",
  //   topic_tags: ["Tree"],
  //   difficulty: "Easy",
  //   status: "active",
  //   prompt_markdown: "# Maximum Depth of Binary Tree\n\nGiven the root of a binary tree...",
  //   created_at: new Date().toISOString(),
  //   updated_at: new Date().toISOString(),
  // }
];

export const getQuestions = async (): Promise<Question[]> => {
  try {
    const response = await getAllQuestions();
    return response.items;
  } catch (error) {
    console.warn("Failed to fetch questions from API, falling back to mock data:", error);
    await new Promise(resolve => setTimeout(resolve, 100));
    return mockQuestions;
  }
};