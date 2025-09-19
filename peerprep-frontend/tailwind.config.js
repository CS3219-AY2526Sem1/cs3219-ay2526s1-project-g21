export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      fontFamily: { sans: ["Inter", "ui-sans-serif", "system-ui"] },
      colors: {
        brand: {
          50:  "#eef2ff",
          100: "#e0e7ff",
          500: "#2F6FED",   // primary
          600: "#215BE0",
          700: "#1B4CBF",
        },
        ink: {
          900: "#0f172a",   // headings
          700: "#334155",   // body
        }
      },
      boxShadow: {
        card: "0 8px 24px rgba(15, 23, 42, 0.06)",
        focus: "0 0 0 3px rgba(47, 111, 237, 0.35)"
      },
      borderRadius: { xl: "14px", "2xl": "20px" }
    }
  },
  plugins: [],
}
