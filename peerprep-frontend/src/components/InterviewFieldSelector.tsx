import React from "react";
import { startCase } from "lodash";

type onFieldChangeCallbackFn = (e: React.ChangeEvent<HTMLSelectElement>) => void;

const InterviewFieldSelector = ({ name, fieldOptions, onChange }: { name: string, fieldOptions: Array<string>, onChange: onFieldChangeCallbackFn }) => {
    return (
        <section className="flex w-full flex-col gap-4">
            <h3 className="text-2xl">
                {startCase(name)}
            </h3>
            <select className="w-full rounded-md border border-gray-300 px-3 py-2 text-base outline-none focus:border-[#2F6FED]" name={name} id={name} defaultValue="" onChange={(e) => onChange(e)}>
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
