import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { Providers } from "@/lib/Providers";
import { ClientLayout } from "@/lib/ClientLayout";

const inter = Inter({ subsets: ["latin"], variable: "--font-inter" });

export const metadata: Metadata = {
  title: "Job Scheduler",
  description: "Distributed Job Scheduler Dashboard",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={inter.className}>
        <Providers>
          <div className="flex" style={{ minHeight: "100vh" }}>
            <ClientLayout>{children}</ClientLayout>
          </div>
        </Providers>
      </body>
    </html>
  );
}
