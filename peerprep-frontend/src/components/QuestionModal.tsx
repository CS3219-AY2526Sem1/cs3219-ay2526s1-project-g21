import { X } from "lucide-react";
import ReactMarkdown from "react-markdown";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { oneDark } from "react-syntax-highlighter/dist/esm/styles/prism";
import type { Question } from "@/types/question";
import { getDifficultyColor } from "@/utils/questionUtils";

interface QuestionModalProps {
  question: Question | null;
  onClose: () => void;
}

export function QuestionModal({ question, onClose }: QuestionModalProps) {
  if (!question) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
    >
      <div
        className="relative w-full max-w-3xl max-h-[90vh] bg-white rounded-lg shadow-xl overflow-hidden flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-start justify-between gap-4 p-6 border-b border-gray-200">
          <div className="flex-1 min-w-0">
            <h2 className="text-xl md:text-2xl font-bold text-gray-900 break-words">
              {question.title}
            </h2>
          </div>
          <div className="flex items-center gap-3 flex-shrink-0">
            <span
              className={`px-2.5 py-0.5 rounded text-xs font-medium whitespace-nowrap ${getDifficultyColor(question.difficulty)}`}
            >
              {question.difficulty}
            </span>
            <button
              onClick={onClose}
              className="p-1 hover:bg-gray-100 rounded-full transition-colors flex-shrink-0"
              aria-label="Close modal"
            >
              <X className="w-5 h-5 text-gray-600" />
            </button>
          </div>
        </div>

        {/* Content - Scrollable */}
        <div className="flex-1 overflow-y-auto p-6 space-y-4">
          {/* Topic Tags */}
          {question.topic_tags && question.topic_tags.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {question.topic_tags.map((tag, index) => (
                <span
                  key={index}
                  className="px-2.5 py-0.5 bg-gray-100 text-gray-700 rounded-full text-xs"
                >
                  {tag.replace(/_/g, " ")}
                </span>
              ))}
            </div>
          )}

          {/* Description Section */}
          <div>
            <h3 className="text-sm font-semibold text-gray-900 mb-2">Description</h3>
            <div className="prose prose-sm max-w-none text-xs">
              <ReactMarkdown
                components={{
                  code({ node, inline, className, children, ...props }) {
                    const match = /language-(\w+)/.exec(className || "");
                    return !inline && match ? (
                      <SyntaxHighlighter
                        style={oneDark}
                        language={match[1]}
                        PreTag="div"
                        customStyle={{ fontSize: '0.75rem' }}
                        {...props}
                      >
                        {String(children).replace(/\n$/, "")}
                      </SyntaxHighlighter>
                    ) : (
                      <code className="text-[0.7rem] bg-gray-100 px-1 py-0.5 rounded" {...props}>
                        {children}
                      </code>
                    );
                  },
                  h1: ({ node, ...props }) => (
                    <h1 className="text-base font-bold text-gray-900 mt-3 mb-1.5" {...props} />
                  ),
                  h2: ({ node, ...props }) => (
                    <h2 className="text-sm font-semibold text-gray-900 mt-2.5 mb-1.5" {...props} />
                  ),
                  h3: ({ node, ...props }) => (
                    <h3 className="text-xs font-semibold text-gray-900 mt-2 mb-1" {...props} />
                  ),
                  p: ({ node, ...props }) => (
                    <p className="text-xs text-gray-700 leading-relaxed mb-1.5" {...props} />
                  ),
                  ul: ({ node, ...props }) => (
                    <ul className="list-disc list-inside space-y-0.5 text-xs text-gray-700 mb-1.5" {...props} />
                  ),
                  ol: ({ node, ...props }) => (
                    <ol className="list-decimal list-inside space-y-0.5 text-xs text-gray-700 mb-1.5" {...props} />
                  ),
                  li: ({ node, ...props }) => (
                    <li className="text-xs text-gray-700 ml-2" {...props} />
                  ),
                  pre: ({ node, ...props }) => (
                    <pre className="bg-gray-50 rounded p-2.5 overflow-x-auto mb-1.5 text-[0.7rem]" {...props} />
                  ),
                  blockquote: ({ node, ...props }) => (
                    <blockquote className="border-l-4 border-gray-300 pl-2.5 italic text-gray-600 text-xs mb-1.5" {...props} />
                  ),
                  strong: ({ node, ...props }) => (
                    <strong className="font-semibold text-gray-900" {...props} />
                  ),
                  em: ({ node, ...props }) => (
                    <em className="italic text-gray-700" {...props} />
                  ),
                  a: ({ node, ...props }) => (
                    <a className="text-blue-600 hover:underline text-xs" {...props} />
                  ),
                }}
              >
                {question.prompt_markdown}
              </ReactMarkdown>
            </div>
          </div>

          {/* Test Cases / Examples */}
          {question.test_cases && question.test_cases.length > 0 && (
            <div>
              <h3 className="text-sm font-semibold text-gray-900 mb-2">Examples</h3>
              <div className="space-y-2.5">
                {question.test_cases.map((testCase, index) => (
                  <div key={index}>
                    <h4 className="text-xs font-semibold text-gray-900 mb-1.5">
                      Example {index + 1}:
                    </h4>
                    <div className="bg-gray-50 rounded p-2.5 space-y-1.5 text-xs font-mono">
                      <div>
                        <span className="font-semibold text-gray-900">Input:</span>{" "}
                        <span className="text-gray-700">{testCase.input}</span>
                      </div>
                      <div>
                        <span className="font-semibold text-gray-900">Output:</span>{" "}
                        <span className="text-gray-700">{testCase.output}</span>
                      </div>
                      {testCase.description && (
                        <div>
                          <span className="font-semibold text-gray-900">Explanation:</span>{" "}
                          <span className="text-gray-700">{testCase.description}</span>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Constraints */}
          {question.constraints && (
            <div>
              <h3 className="text-sm font-semibold text-gray-900 mb-2">Constraints</h3>
              <div className="bg-gray-50 rounded p-2.5">
                <ul className="space-y-0.5 text-xs text-gray-700 font-mono">
                  {question.constraints.split('\n').map((constraint, index) => (
                    <li key={index} className="flex items-start">
                      <span className="mr-2">•</span>
                      <span className="flex-1">{constraint}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
