import { useState, useEffect } from "react";

export const useFileDrag = (ref: React.RefObject<HTMLElement>) => {
  const [isDragActive, setIsDragActive] = useState(false);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;

    let dragCounter = 0;

    const handleDragEnter = (e: DragEvent) => {
      e.preventDefault();
      dragCounter++;
      if (e.dataTransfer?.types) {
        const isFile = Array.from(e.dataTransfer.types).includes("Files");
        if (isFile) {
          setIsDragActive(true);
        }
      }
    };

    const handleDragLeave = (e: DragEvent) => {
      e.preventDefault();
      dragCounter--;
      if (dragCounter === 0) {
        setIsDragActive(false);
      }
    };

    const handleDragOver = (e: DragEvent) => {
      e.preventDefault();
      if (e.dataTransfer) {
        e.dataTransfer.dropEffect = "copy";
      }
    };

    const handleDrop = (e: DragEvent) => {
      e.preventDefault();
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
