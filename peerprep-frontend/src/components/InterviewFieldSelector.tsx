import React from "react";
import { startCase } from "lodash";

type onFieldChangeCallbackFn = (e: React.ChangeEvent<HTMLSelectElement>) => void;

const InterviewFieldSelector = ({ name, fieldOptions, onChange }: { name: string, fieldOptions: Array<string>, onChange: onFieldChangeCallbackFn }) => {
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