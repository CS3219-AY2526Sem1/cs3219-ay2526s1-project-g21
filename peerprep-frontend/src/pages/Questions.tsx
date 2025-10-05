import { useState } from "react";

interface Question {
  id: number;
  title: string;
  topic: string;
  difficulty: "Easy" | "Medium" | "Hard";
  status: "Solved" | "Attempted" | "Unsolved";
}

type Difficulty = Question["difficulty"];
type Status = Question["status"];

// utils stuff
const getDifficultyColor = (difficulty: Difficulty): string => {
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

const getStatusColor = (status: Status): string => {
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

type Difficulty = Question["difficulty"];
type Status = Question["status"];

