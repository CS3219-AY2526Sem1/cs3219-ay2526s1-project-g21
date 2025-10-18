import hero from "@/assets/jose.png";
import { Link } from "react-router-dom";

export default function Home() {
  return (
    <section className="mx-auto max-w-5xl px-6 py-14 text-center">
      <h1 className="text-[40px] md:text-[56px] leading-[1.1] font-semibold text-black">
        Interview prep made easy
      </h1>

      <p className="mt-6 text-lg md:text-xl text-[#4B5563] max-w-3xl mx-auto">
        PeerPrep makes it easier than ever to prepare for tech interviews.
        Simply pick a difficulty, pick a question category, and start practicing!
      </p>

      <div className="mt-12">
        <img
          src={hero}
          alt="Two people practicing interview"
          className="mx-auto w-full max-w-3xl"
        />
      </div>

      <div className="mt-10 flex items-center justify-center gap-4">
        <Link
          to="/interview"
          className="inline-flex items-center justify-center rounded-md bg-[#2F6FED] px-6 py-3 text-white text-base font-medium hover:brightness-95"
        >
          Start Practicing
        </Link>
        <Link
          to="/questions"
          className="inline-flex items-center justify-center rounded-md border border-[#D1D5DB] px-6 py-3 text-base font-medium text-black hover:bg-gray-50"
        >
          View Questions
        </Link>
      </div>
    </section>
  );
}
