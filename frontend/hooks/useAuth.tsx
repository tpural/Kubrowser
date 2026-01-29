"use client";

import { useState, useEffect, createContext, useContext } from 'react';

interface User {
    login: string;
    avatar_url: string;
}

interface AuthContextType {
    user: User | null;
    loading: boolean;
    login: () => void;
    logout: () => void;
}

const AuthContext = createContext<AuthContextType>({
    user: null,
    loading: true,
    login: () => { },
    logout: () => { },
});

export function AuthProvider({ children }: { children: React.ReactNode }) {
    const [user, setUser] = useState<User | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        const fetchUser = async () => {
            try {
                const protocol = window.location.protocol === "https:" ? "https:" : "http:";
                const host = process.env.NEXT_PUBLIC_API_URL
                    ? process.env.NEXT_PUBLIC_API_URL.replace(/^https?:\/\//, "").replace(/\/$/, "")
                    : "localhost:8080";

                const res = await fetch(`${protocol}//${host}/auth/me`, {
                    credentials: 'include' // Include cookies in cross-origin requests
                });
                if (res.ok) {
                    const data = await res.json();
                    setUser(data);
                }
            } catch (error) {
                console.error("Failed to fetch user:", error);
            } finally {
                setLoading(false);
            }
        };

        fetchUser();
    }, []);

    const login = () => {
        const protocol = window.location.protocol === "https:" ? "https:" : "http:";
        const host = process.env.NEXT_PUBLIC_API_URL
            ? process.env.NEXT_PUBLIC_API_URL.replace(/^https?:\/\//, "").replace(/\/$/, "")
            : "localhost:8080";
        window.location.href = `${protocol}//${host}/auth/login`;
    };

    const logout = () => {
        const protocol = window.location.protocol === "https:" ? "https:" : "http:";
        const host = process.env.NEXT_PUBLIC_API_URL
            ? process.env.NEXT_PUBLIC_API_URL.replace(/^https?:\/\//, "").replace(/\/$/, "")
            : "localhost:8080";
        window.location.href = `${protocol}//${host}/auth/logout`;
    };

    return (
        <AuthContext.Provider value={{ user, loading, login, logout }}>
            {children}
        </AuthContext.Provider>
    );
}

export function useAuth() {
    return useContext(AuthContext);
}
