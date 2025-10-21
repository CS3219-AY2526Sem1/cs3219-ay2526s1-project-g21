import hero from "@/assets/jose.png";
import { Link } from "react-router-dom";

export default function Home() {
  return (
    <section className="mx-auto max-w-5xl px-4 py-14 text-center sm:px-6">
      <h1 className="text-4xl font-semibold leading-tight text-black md:text-6xl md:leading-[1.1]">
        Interview prep made easy
      </h1>

      <p className="mx-auto mt-6 max-w-3xl text-base text-[#4B5563] sm:text-lg md:text-xl">
        PeerPrep makes it easier than ever to prepare for tech interviews.
        Simply pick a difficulty, pick a question category, and start practicing!
      </p>

      <div className="mt-12">
        <img
          src={hero}
          alt="Two people practicing interview"
          className="mx-auto w-full max-w-3xl rounded-xl shadow-sm"
        />
      </div>

      <div className="mt-10 flex flex-col items-center gap-3 sm:flex-row sm:justify-center sm:gap-4">
        <Link
          to="/interview"
          className="inline-flex w-full items-center justify-center rounded-md bg-[#2F6FED] px-6 py-3 text-base font-medium text-white hover:brightness-95 sm:w-auto"
        >
          Start Practicing
        </Link>
        <Link
          to="/questions"
          className="inline-flex w-full items-center justify-center rounded-md border border-[#D1D5DB] px-6 py-3 text-base font-medium text-black hover:bg-gray-50 sm:w-auto"
        >
          View Questions
        </Link>
      </div>
    </section>
  );
}
