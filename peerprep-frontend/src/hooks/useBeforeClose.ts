import { useEffect } from "react";

export default function useBeforeClose(onClose: () => void) {
    useEffect(() => {
        const handleBeforeUnload = () => {
            onClose();
        };

        // Fires when the user is about to close/refresh/navigate away
        window.addEventListener("beforeunload", handleBeforeUnload);

        return () => {
            window.removeEventListener("beforeunload", handleBeforeUnload);
        };
    }, [onClose]);
}
