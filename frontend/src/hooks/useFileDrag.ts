import { useState, useEffect } from "react";

export const useFileDrag = (ref: React.RefObject<HTMLElement>) => {
  const [isDragActive, setIsDragActive] = useState(false);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;

    let dragCounter = 0;

    const handleDragEnter = (e: DragEvent) => {
      e.preventDefault();
      // e.stopPropagation();
      dragCounter++;
      if (e.dataTransfer?.types) {
        // Check if it's a file drag
        const isFile = Array.from(e.dataTransfer.types).includes("Files");
        if (isFile) {
          setIsDragActive(true);
        }
      }
    };

    const handleDragLeave = (e: DragEvent) => {
      e.preventDefault();
      // e.stopPropagation();
      dragCounter--;
      if (dragCounter === 0) {
        setIsDragActive(false);
      }
    };

    const handleDragOver = (e: DragEvent) => {
      e.preventDefault();
      // e.stopPropagation();
      if (e.dataTransfer) {
        e.dataTransfer.dropEffect = "copy";
      }
    };

    const handleDrop = (e: DragEvent) => {
      e.preventDefault();
      // Do not stop propagation to allow Wails runtime or other listeners to catch the drop
      // e.stopPropagation();
      setIsDragActive(false);
      dragCounter = 0;
    };

    element.addEventListener("dragenter", handleDragEnter);
    element.addEventListener("dragleave", handleDragLeave);
    element.addEventListener("dragover", handleDragOver);
    element.addEventListener("drop", handleDrop);

    return () => {
      element.removeEventListener("dragenter", handleDragEnter);
      element.removeEventListener("dragleave", handleDragLeave);
      element.removeEventListener("dragover", handleDragOver);
      element.removeEventListener("drop", handleDrop);
    };
  }, [ref]);

  return isDragActive;
};
