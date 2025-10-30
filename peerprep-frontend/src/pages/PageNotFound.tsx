import hero from "@/assets/pageNotFound.png";
import { Link } from "react-router-dom";

export default function PageNotFound() {
    return (
        <section className="mx-auto max-w-5xl px-6 py-14 text-center">
            <div className="mt-12">
                <img
                    src={hero}
                    alt="Two people practicing interview"
                    className="mx-auto w-1/2 max-w-lg"
                />
            </div>

            <div>
                <h1 className="text-3xl font-semibold text-black">
                    Requested Page Not Found
                </h1>
            </div>

            <div className="mt-10 flex items-center justify-center gap-4">
                <Link
                    to="/"
                    className="inline-flex items-center justify-center rounded-md bg-[#2F6FED] px-6 py-3 text-white text-base font-medium hover:brightness-95"
                >
                    Return to Home Page
                </Link>
            </div>
        </section>
    );
}
