/**
 * Site Theme Manager
 * Manages site-wide themes like Christmas, Halloween, etc.
 * Themes are controlled by admins and apply to all users.
 */

interface ThemeEffects {
    init(): void;
    cleanup(): void;
}

class ChristmasTheme implements ThemeEffects {
    private container: HTMLElement | null = null;
    private snowflakeInterval: number | null = null;
    private maxSnowflakes = 50;
    private snowflakeChars = ["❄", "❅", "❆", "✻", "✼"];

    init(): void {
        // Create container for theme effects
        this.container = document.createElement("div");
        this.container.id = "theme-effects-container";
        document.body.appendChild(this.container);

        // Start creating snowflakes
        this.createSnowflake();
        this.snowflakeInterval = window.setInterval(() => this.createSnowflake(), 200);
    }

    private createSnowflake(): void {
        if (!this.container) return;

        // Limit the number of snowflakes
        const existingSnowflakes = this.container.querySelectorAll(".snowflake");
        if (existingSnowflakes.length >= this.maxSnowflakes) return;

        const snowflake = document.createElement("div");
        snowflake.className = "snowflake";
        snowflake.textContent = this.snowflakeChars[Math.floor(Math.random() * this.snowflakeChars.length)];

        // Random position and animation properties
        const startX = Math.random() * 100;
        const size = Math.random() * 0.8 + 0.5; // 0.5 to 1.3 em
        const duration = Math.random() * 5 + 8; // 8 to 13 seconds
        const delay = Math.random() * 2;

        snowflake.style.left = `${startX}%`;
        snowflake.style.fontSize = `${size}em`;
        snowflake.style.animationDuration = `${duration}s`;
        snowflake.style.animationDelay = `${delay}s`;
        snowflake.style.opacity = `${Math.random() * 0.4 + 0.4}`; // 0.4 to 0.8

        this.container.appendChild(snowflake);

        // Remove snowflake after animation completes
        setTimeout(
            () => {
                snowflake.remove();
            },
            (duration + delay) * 1000,
        );
    }

    cleanup(): void {
        if (this.snowflakeInterval) {
            clearInterval(this.snowflakeInterval);
            this.snowflakeInterval = null;
        }
        if (this.container) {
            this.container.remove();
            this.container = null;
        }
    }
}

class SiteThemeManager {
    private static instance: SiteThemeManager | null = null;
    private currentTheme: string = "";
    private currentEffects: ThemeEffects | null = null;

    private themeEffects: Record<string, () => ThemeEffects> = {
        christmas: () => new ChristmasTheme(),
    };

    private constructor() {}

    static getInstance(): SiteThemeManager {
        if (!SiteThemeManager.instance) {
            SiteThemeManager.instance = new SiteThemeManager();
        }
        return SiteThemeManager.instance;
    }

    async init(): Promise<void> {
        try {
            const response = await fetch("/api/theme/active");
            if (response.ok) {
                const data = await response.json();
                this.applyTheme(data.themeId || "");
            }
        } catch (error) {
            console.error("Failed to fetch active theme:", error);
        }
    }

    applyTheme(themeId: string): void {
        // If same theme is already applied, do nothing
        if (this.currentTheme === themeId) return;

        // Cleanup previous theme effects
        if (this.currentEffects) {
            this.currentEffects.cleanup();
            this.currentEffects = null;
        }

        // Remove old theme class from body
        if (this.currentTheme) {
            document.body.classList.remove(`theme-${this.currentTheme}`);
        }

        this.currentTheme = themeId;

        // Apply new theme
        if (themeId) {
            document.body.classList.add(`theme-${themeId}`);

            // Initialize theme effects if available
            const effectsFactory = this.themeEffects[themeId];
            if (effectsFactory) {
                this.currentEffects = effectsFactory();
                this.currentEffects.init();
            }
        }
    }

    getCurrentTheme(): string {
        return this.currentTheme;
    }
}

// Export for use in other modules
export const siteThemeManager = SiteThemeManager.getInstance();

// Auto-initialize when DOM is ready
if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => siteThemeManager.init());
} else {
    siteThemeManager.init();
}
