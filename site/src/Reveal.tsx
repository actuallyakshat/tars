import { motion } from "motion/react";
import type { ReactNode } from "react";

// A small rise-and-sharpen as content first scrolls into view. Runs once.
export function Reveal({
  children,
  delay = 0,
  className,
  as = "div",
}: {
  children: ReactNode;
  delay?: number;
  className?: string;
  as?: "div" | "li";
}) {
  const Tag = as === "li" ? motion.li : motion.div;
  return (
    <Tag
      className={className}
      initial={{ opacity: 0, y: 10, filter: "blur(3px)" }}
      whileInView={{ opacity: 1, y: 0, filter: "blur(0px)" }}
      viewport={{ once: true, margin: "0px 0px -12% 0px" }}
      transition={{ duration: 0.6, delay, ease: [0.2, 0.7, 0.2, 1] }}
    >
      {children}
    </Tag>
  );
}

// Buttons and links lift slightly on hover and settle on press.
export const press = {
  whileHover: { y: -1 },
  whileTap: { y: 0, scale: 0.98 },
  transition: { type: "spring", stiffness: 500, damping: 30 },
} as const;
