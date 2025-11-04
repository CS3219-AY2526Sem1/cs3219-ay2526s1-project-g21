import { useState, useEffect } from "react";
import { ExternalLink, Flag } from "lucide-react";
import type { Question } from "@/types/question";
import { getDifficultyColor } from "@/utils/questionUtils";
import { getQuestions } from "@/services/questionService";

const DEFAULT_ITEMS_PER_PAGE = 10;

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
  const [currentPage, setCurrentPage] = useState(1);
  const [itemsPerPage] = useState(DEFAULT_ITEMS_PER_PAGE);
  const [totalPages, setTotalPages] = useState(1);
  const [totalItems, setTotalItems] = useState(0);
  const [hasNext, setHasNext] = useState(false);
  const [hasPrev, setHasPrev] = useState(false);

  const getStartItem = () => totalItems === 0 ? 0 : (currentPage - 1) * itemsPerPage + 1;
  const getEndItem = () => Math.min(currentPage * itemsPerPage, totalItems);

  useEffect(() => {
    const loadQuestions = async () => {
      try {
        setLoading(true);
        const data = await getQuestions(currentPage, itemsPerPage);
        setQuestions(data.items);
        setTotalPages(data.totalPages);
        setTotalItems(data.total);
        setHasNext(data.hasNext);
        setHasPrev(data.hasPrev);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load questions');
      } finally {
        setLoading(false);
      }
    };

    loadQuestions();
  }, [currentPage, itemsPerPage]);

  if (loading) {
    return (
      <section className="mx-auto max-w-7xl px-4 py-14 sm:px-6">
        <div className="text-center">Loading questions...</div>
      </section>
    );
  }

  if (error) {
    return (
      <section className="mx-auto max-w-7xl px-4 py-14 sm:px-6">
        <div className="text-center text-red-600">Error: {error}</div>
      </section>
    );
  }

  return (
    <section className="mx-auto max-w-7xl px-4 py-14 sm:px-6">
      <div className="mb-8 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="text-3xl font-semibold text-black">All Questions</h1>
        <div className="flex w-full flex-col gap-3 sm:w-auto sm:flex-row sm:items-center sm:gap-4">
          <button className="inline-flex w-full items-center justify-center rounded-md bg-[#2F6FED] px-4 py-2 text-sm font-medium text-white hover:brightness-95 sm:w-auto">
            Filter Questions
          </button>
          <input
            type="text"
            placeholder="Search Questions"
            className="w-full rounded-md border border-[#D1D5DB] px-3 py-2 text-sm focus:border-transparent focus:outline-none focus:ring-2 focus:ring-[#2F6FED] sm:w-64"
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

        <div className="flex flex-col gap-3 border-t border-[#E5E7EB] bg-white px-4 py-3 text-center sm:flex-row sm:items-center sm:justify-between sm:px-6 sm:text-left">
          <div className="text-sm text-gray-700 sm:text-left">
            Showing {getStartItem()} to {getEndItem()} of {totalItems} rows
          </div>
          <div className="flex items-center justify-center gap-2 sm:justify-end">
            <button 
              onClick={() => setCurrentPage(prev => prev - 1)}
              disabled={!hasPrev || loading}
              className="rounded-md border border-[#D1D5DB] px-3 py-2 text-sm hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
            >
              Previous
            </button>
            <span className="px-3 py-2 text-sm text-gray-700">
              Page {currentPage} of {totalPages}
            </span>
            <button 
              onClick={() => setCurrentPage(prev => prev + 1)}
              disabled={!hasNext || loading}
              className="rounded-md border border-[#D1D5DB] px-3 py-2 text-sm hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
            >
              Next
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}
