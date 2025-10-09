import type { Difficulty, Status } from "@/types/question";

export const getDifficultyColor = (difficulty: Difficulty): string => {
  switch (difficulty) {
    case "Easy":
      return "text-green-600";
    case "Medium":
      return "text-yellow-600";
    case "Hard":
      return "text-red-600";
    default:
      return "text-gray-600";
  }
};

export const getStatusColor = (status: Status): string => {
  switch (status) {
    case "Solved":
      return "text-green-600";
    case "Attempted":
      return "text-yellow-600";
    case "Unsolved":
      return "text-red-600";
    default:
      return "text-gray-600";
  }
};