"use client";

import { motion } from "framer-motion";

interface LoadingSpinnerProps {
  className?: string;
  size?: "sm" | "md" | "lg";
  text?: string;
}

export function LoadingSpinner({
  className,
  size = "md",
  text,
}: LoadingSpinnerProps) {
  const sizeClasses = {
    sm: "h-6 w-6",
    md: "h-12 w-12",
    lg: "h-16 w-16",
  };

  const borderSizes = {
    sm: "border-2",
    md: "border-4",
    lg: "border-4",
  };

  return (
    <div
      className={`flex flex-col items-center justify-center gap-4 ${className || ""}`}
    >
      <motion.div
        className="relative"
        initial={{ opacity: 0, scale: 0.8 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.3 }}
      >
        {/* Outer ring */}
        <div
          className={`${sizeClasses[size]} rounded-full ${borderSizes[size]} border-muted`}
        />

        {/* Spinning gradient ring */}
        <motion.div
          className={`absolute top-0 left-0 ${sizeClasses[size]} rounded-full ${borderSizes[size]} border-transparent`}
          style={{
            borderTopColor: "hsl(var(--primary))",
            borderRightColor: "hsl(var(--primary) / 0.5)",
          }}
          animate={{ rotate: 360 }}
          transition={{
            duration: 1,
            repeat: Infinity,
            ease: "linear",
          }}
        />

        {/* Glowing center dot */}
        <motion.div
          className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary"
          style={{
            width: size === "sm" ? 4 : size === "md" ? 6 : 8,
            height: size === "sm" ? 4 : size === "md" ? 6 : 8,
          }}
          animate={{
            scale: [1, 1.2, 1],
            opacity: [0.7, 1, 0.7],
          }}
          transition={{
            duration: 1.5,
            repeat: Infinity,
            ease: "easeInOut",
          }}
        />
      </motion.div>

      {text && (
        <motion.p
          initial={{ opacity: 0, y: 5 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2 }}
          className="text-sm text-muted-foreground"
        >
          {text}
        </motion.p>
      )}
    </div>
  );
}
