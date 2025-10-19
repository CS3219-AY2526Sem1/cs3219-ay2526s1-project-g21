import React from "react";
import { Difficulty, Category } from "@/types/question";
import { startCase } from "lodash";

type OnFieldChangeCallbackFn = (e: React.ChangeEvent<HTMLSelectElement>) => void;

interface InterviewFieldSelectorProps {
    name: string;
    fieldOptions: readonly Difficulty[] | readonly Category[];
    onChange: OnFieldChangeCallbackFn;
}

const InterviewFieldSelector: React.FC<InterviewFieldSelectorProps> = ({ name, fieldOptions, onChange }) => {
    return (
        <section className="flex flex-col gap-4">
            <h3 className="text-2xl">
                {startCase(name)}
            </h3>
            <select className="outline outline-1 outline-gray-600 rounded-md px-2 py-1.5 text-base" name={name} id={name} defaultValue="" onChange={(e) => onChange(e)}>
                {
                    fieldOptions.map((x: string) => (
                        <option value={x} key={x}>
                            {startCase(x)}
                        </option>
                    ))
                }
            </select>
        </section>
    );
};

export default InterviewFieldSelector;