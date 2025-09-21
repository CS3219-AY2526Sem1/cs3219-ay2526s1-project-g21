import { ChangeEvent } from "react";

export type FormSetter<T> = React.Dispatch<React.SetStateAction<T>>;

export function handleFormChange<T>(
  e: ChangeEvent<HTMLInputElement>,
  setForm: FormSetter<T>
) {
  const { name, value } = e.target;
  setForm((prev) => ({ ...prev, [name]: value }));
}
