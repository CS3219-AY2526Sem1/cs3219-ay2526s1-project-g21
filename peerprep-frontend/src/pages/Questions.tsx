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

interface QuestionsTableRowProps {
  question: Question;
}

const QuestionsTableRow = ({ question }: QuestionsTableRowProps) => {
  return (
    <tr>
      <td>{question.title}</td>
      <td>{question.topic}</td>
      <td>{question.difficulty}</td>
      <td>{question.status}</td>
      <td>
        <button>View</button>
        <button>Play</button>
      </td>
    </tr>
  );
};

// mock data for testing
const mockQuestions: Question[] = [
  {
    id: 1,
    title: "Reverse a Linked List",
    topic: "Linked List",
    difficulty: "Hard",
    status: "Solved"
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
    title: "Jungwoo and Bananas",
    topic: "Greedy",
    difficulty: "Hard",
    status: "Unsolved"
  },
  {
    id: 6,
    title: "Minimum Triangulation Score of Polygon",
    topic: "Math",
    difficulty: "Medium",
    status: "Unsolved"
  }
];

export default function Questions() {
  return (
    <div>
      <h1>All Questions</h1>
      <table>
        <thead>
          <tr>
            <th>Question</th>
            <th>Topic</th>
            <th>Difficulty</th>
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {mockQuestions.map((question) => (
            <QuestionsTableRow key={question.id} question={question} />
          ))}
        </tbody>
      </table>
    </div>
  );
}