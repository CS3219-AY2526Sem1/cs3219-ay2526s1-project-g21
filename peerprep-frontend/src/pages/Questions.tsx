import { useState } from "react";
import { ExternalLink, Flag } from "lucide-react";
import type { Question } from "@/types/question";
import { getDifficultyColor, getStatusColor } from "@/utils/questionUtils";

interface QuestionsTableRowProps {
  question: Question;
}

const QuestionsTableRow = ({ question }: QuestionsTableRowProps) => {
  return (
    <tr className="hover:bg-gray-50">
      <td className="px-6 py-4 text-sm text-gray-900">{question.title}</td>
      <td className="px-6 py-4 text-sm text-gray-600">{question.topic}</td>
      <td className={`px-6 py-4 text-sm font-medium ${getDifficultyColor(question.difficulty)}`}>
        {question.difficulty}
      </td>
      <td className={`px-6 py-4 text-sm font-medium ${getStatusColor(question.status)}`}>
        {question.status}
      </td>
      <td className="px-6 py-4 text-sm">
        <div className="flex items-center gap-2">
          <button className="p-1 hover:bg-gray-100 rounded">
            <ExternalLink className="w-4 h-4 text-gray-600" />
          </button>
          <button className="p-1 hover:bg-gray-100 rounded">
            <Flag className="w-4 h-4 text-gray-600" />
          </button>
        </div>
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
    <section className="mx-auto max-w-7xl px-6 py-14">
      <h1 className="text-3xl font-semibold text-black mb-8">All Questions</h1>

      <div className="bg-white border border-[#E5E7EB] rounded-lg overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 border-b border-[#E5E7EB]">
              <tr>
                <th className="px-6 py-3 text-left text-sm font-medium text-gray-900">Question</th>
                <th className="px-6 py-3 text-left text-sm font-medium text-gray-900">Topic</th>
                <th className="px-6 py-3 text-left text-sm font-medium text-gray-900">Difficulty</th>
                <th className="px-6 py-3 text-left text-sm font-medium text-gray-900">Status</th>
                <th className="px-6 py-3 text-left text-sm font-medium text-gray-900">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[#E5E7EB]">
              {mockQuestions.map((question) => (
                <QuestionsTableRow key={question.id} question={question} />
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}