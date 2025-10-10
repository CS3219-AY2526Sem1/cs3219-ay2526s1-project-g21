import { useState, useEffect } from "react";
import { ExternalLink, Flag } from "lucide-react";
import type { Question } from "@/types/question";
import { getDifficultyColor } from "@/utils/questionUtils";
import { getQuestions } from "@/services/questionService";

interface QuestionsTableRowProps {
  question: Question;
}

const QuestionsTableRow = ({ question }: QuestionsTableRowProps) => {
  return (
    <tr className="hover:bg-gray-50">
      <td className="px-6 py-4 text-sm text-gray-900">{question.title}</td>
      <td className="px-6 py-4 text-sm text-gray-600">{question.topic_tags?.join(", ") || "No topics"}</td>
      <td className={`px-6 py-4 text-sm font-medium ${getDifficultyColor(question.difficulty)}`}>
        {question.difficulty}
      </td>
      <td className="px-6 py-4 text-sm font-medium text-gray-500">
        Unsolved
      </td>
      <td className="px-6 py-4 text-sm">
        <div className="flex items-center gap-2">
          <button
            className="p-1 hover:bg-gray-100 rounded"
            aria-label="Open question"
            title="Open question"
            type="button"
          >
            <ExternalLink className="w-4 h-4 text-gray-600" />
          </button>
          <button
            className="p-1 hover:bg-gray-100 rounded"
            aria-label="Report question"
            title="Report question"
            type="button"
          >
            <Flag className="w-4 h-4 text-gray-600" />
          </button>
        </div>
      </td>
    </tr>
  );
};

export default function Questions() {
  const [questions, setQuestions] = useState<Question[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadQuestions = async () => {
      try {
        setLoading(true);
        const data = await getQuestions();
        setQuestions(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load questions');
      } finally {
        setLoading(false);
      }
    };

    loadQuestions();
  }, []);

  if (loading) {
    return (
      <section className="mx-auto max-w-7xl px-6 py-14">
        <div className="text-center">Loading questions...</div>
      </section>
    );
  }

  if (error) {
    return (
      <section className="mx-auto max-w-7xl px-6 py-14">
        <div className="text-center text-red-600">Error: {error}</div>
      </section>
    );
  }

  return (
    <section className="mx-auto max-w-7xl px-6 py-14">
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-3xl font-semibold text-black">All Questions</h1>
        <div className="flex items-center gap-4">
          <button className="inline-flex items-center justify-center rounded-md bg-[#2F6FED] px-4 py-2 text-white text-sm font-medium hover:brightness-95">
            Filter Questions
          </button>
          <input
            type="text"
            placeholder="Search Questions"
            className="rounded-md border border-[#D1D5DB] px-3 py-2 text-sm w-64 focus:outline-none focus:ring-2 focus:ring-[#2F6FED] focus:border-transparent"
          />
        </div>
      </div>

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
              {questions.map((question) => (
                <QuestionsTableRow key={question.id} question={question} />
              ))}
            </tbody>
          </table>
        </div>

        <div className="px-6 py-3 bg-white border-t border-[#E5E7EB] flex items-center justify-between">
          <div className="text-sm text-gray-700">
            Showing 1 to {Math.min(6, questions.length)} of {questions.length} rows
          </div>
          <div className="flex items-center gap-2">
            <button className="px-3 py-2 text-sm border border-[#D1D5DB] rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed">
              Previous
            </button>
            <button className="px-3 py-2 text-sm border border-[#D1D5DB] rounded-md hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed">
              Next
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}