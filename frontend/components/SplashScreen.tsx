"use client";

import { motion } from "framer-motion";
import { Github, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";

interface SplashScreenProps {
    onLogin: () => void;
    loading?: boolean;
}

export function SplashScreen({ onLogin, loading = false }: SplashScreenProps) {
    return (
        <div className="flex flex-col items-center justify-center min-h-[calc(100vh-80px)] w-full relative overflow-hidden">
            {/* Background decorations */}
            <div className="absolute top-0 left-0 w-full h-full overflow-hidden pointer-events-none z-0 opacity-50">
                <div className="absolute top-[10%] left-[20%] w-[40%] h-[40%] bg-blue-500/10 rounded-full blur-[100px] animate-pulse" />
                <div className="absolute bottom-[20%] right-[20%] w-[30%] h-[30%] bg-purple-500/10 rounded-full blur-[100px] animate-float" />
            </div>

            <div className="relative z-10 text-center space-y-8 max-w-2xl px-4">
                <motion.div
                    initial={{ scale: 0.8, opacity: 0 }}
                    animate={{ scale: 1, opacity: 1 }}
                    transition={{ type: "spring", stiffness: 200, damping: 20 }}
                    className="mx-auto relative w-32 h-32 mb-6"
                >
                    <div className="absolute inset-0 bg-gradient-to-tr from-blue-500/20 to-purple-500/20 rounded-full blur-xl" />
                    <img
                        src="/logo.svg"
                        alt="Logo"
                        className="relative w-full h-full object-contain drop-shadow-2xl"
                    />
                </motion.div>

                <motion.div
                    initial={{ y: 20, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.2 }}
                    className="space-y-4"
                >
                    <h1 className="text-4xl md:text-6xl font-bold tracking-tight bg-gradient-to-r from-[#326CE5] via-[#5B8FF9] to-[#326CE5] bg-clip-text text-transparent bg-[length:200%_auto] animate-gradient" style={{ fontFamily: 'var(--font-jetbrains-mono)' }}>
                        Kubrowser
                    </h1>
                    <p className="text-lg md:text-xl text-muted-foreground max-w-lg mx-auto leading-relaxed">
                        Secure, ephemeral Kubernetes access directly from your browser.
                        <br />
                        <span className="text-sm opacity-75">Connect to pods, view logs, and execute commands instantly.</span>
                    </p>
                </motion.div>

                <motion.div
                    initial={{ y: 20, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.4 }}
                    className="pt-8"
                >
                    {loading ? (
                        <div className="flex flex-col items-center gap-3">
                            <div className="h-6 w-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
                            <span className="text-sm text-muted-foreground animate-pulse">Initializing session...</span>
                        </div>
                    ) : (
                        <div className="flex flex-col gap-4 items-center">
                            <Button
                                size="lg"
                                onClick={onLogin}
                                className="h-14 px-8 text-lg font-medium shadow-lg hover:shadow-primary/20 hover:scale-105 transition-all duration-300 bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-700 hover:to-indigo-700"
                            >
                                <Github className="mr-3 h-5 w-5" />
                                Connect with GitHub
                            </Button>
                            <div className="flex items-center gap-2 text-xs text-muted-foreground/60">
                                <Sparkles className="h-3 w-3" />
                                <span>Authentication Required</span>
                            </div>
                        </div>
                    )}
                </motion.div>
            </div>

            <div className="absolute bottom-8 text-center text-sm text-muted-foreground/40">
                <p>Kubrowser v1.0.0 • Secure • Ephemeral</p>
            </div>
        </div>
    );
}
