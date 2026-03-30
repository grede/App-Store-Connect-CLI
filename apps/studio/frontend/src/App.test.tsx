import { fireEvent, render, screen } from "@testing-library/react";

import App from "./App";

describe("App", () => {
  it("shows persistent app context bar", () => {
    render(<App />);

    expect(screen.getByText("MusadoraKit", { selector: ".context-app-name" })).toBeInTheDocument();
    expect(screen.getByText("iOS", { selector: ".context-badge" })).toBeInTheDocument();
    expect(screen.getByText(/v2\.3\.0/, { selector: ".context-version" })).toBeInTheDocument();
  });

  it("switches workspace sections from the sidebar", () => {
    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: /builds/i }));

    // "Builds" appears in sidebar and section label
    expect(screen.getAllByText("Builds").length).toBeGreaterThanOrEqual(2);
  });

  it("sends a chat message and expands the dock", () => {
    render(<App />);

    const textarea = screen.getByLabelText("Chat prompt");
    fireEvent.change(textarea, { target: { value: "list builds" } });
    fireEvent.submit(textarea.closest("form")!);

    expect(screen.getByText("list builds")).toBeInTheDocument();
    expect(screen.getByText(/bootstrap mode/i)).toBeInTheDocument();
    expect(screen.getByText("ACP Chat")).toBeInTheDocument();
  });

  it("collapses the dock when chevron is clicked", () => {
    render(<App />);

    const textarea = screen.getByLabelText("Chat prompt");
    fireEvent.change(textarea, { target: { value: "test" } });
    fireEvent.submit(textarea.closest("form")!);

    expect(screen.getByText("ACP Chat")).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Collapse chat"));

    expect(screen.queryByText("ACP Chat")).not.toBeInTheDocument();
  });
});
