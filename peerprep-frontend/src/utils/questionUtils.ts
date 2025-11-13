import type { Question } from "@/types/question";

export const getDifficultyColor = (difficulty: Question["difficulty"]): string => {
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