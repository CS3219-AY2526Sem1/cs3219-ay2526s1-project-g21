import type { Question } from "@/types/question";

const mockQuestions: Question[] = [
  {
    id: 1,
    title: "Jungwoo and his Bananas",
    topic: "Greedy",
    difficulty: "Hard",
    status: "Unsolved"
  },
  {
    id: 2,
    title: "Find Duplicate Peak Element in Array",
    topic: "Binary Search",
    difficulty: "Medium",
    status: "Attempted"
  },
  {
    id: 3,
    title: "House Robbers I",
    topic: "Dynamic Programming",
    difficulty: "Easy",
    status: "Unsolved"
  },
  {
    id: 4,
    title: "Gas Stations",
    topic: "Dynamic Programming",
    difficulty: "Medium",
    status: "Unsolved"
  },
  {
    id: 5,
    title: "Reverse a Linked List",
    topic: "Linked List",
    difficulty: "Hard",
    status: "Solved"
  },
  {
    id: 6,
    title: "Minimum Triangulation Score of Polygon",
    topic: "Math",
    difficulty: "Medium",
    status: "Unsolved"
  },
  {
    id: 7,
    title: "Two Sum",
    topic: "Hash Table",
    difficulty: "Easy",
    status: "Solved"
  },
  {
    id: 8,
    title: "Valid Parentheses",
    topic: "Stack",
    difficulty: "Easy",
    status: "Solved"
  },
  {
    id: 9,
    title: "Merge Two Sorted Lists",
    topic: "Linked List",
    difficulty: "Easy",
    status: "Attempted"
  },
  {
    id: 10,
    title: "Best Time to Buy and Sell Stock",
    topic: "Array",
    difficulty: "Easy",
    status: "Unsolved"
  },
  {
    id: 11,
    title: "Maximum Subarray",
    topic: "Dynamic Programming",
    difficulty: "Medium",
    status: "Solved"
  },
  {
    id: 12,
    title: "Climbing Stairs",
    topic: "Dynamic Programming",
    difficulty: "Easy",
    status: "Solved"
  },
  {
    id: 13,
    title: "Binary Tree Inorder Traversal",
    topic: "Tree",
    difficulty: "Easy",
    status: "Attempted"
  },
  {
    id: 14,
    title: "Symmetric Tree",
    topic: "Tree",
    difficulty: "Easy",
    status: "Unsolved"
  },
  {
    id: 15,
    title: "Maximum Depth of Binary Tree",
    topic: "Tree",
    difficulty: "Easy",
    status: "Solved"
  }
];

// send back mock data for now.
// TODO: Future API implementation - update when microservice is ready
export const getQuestions = async (): Promise<Question[]> => {
  // Simulate API delay
  await new Promise(resolve => setTimeout(resolve, 100));
  return mockQuestions;
};