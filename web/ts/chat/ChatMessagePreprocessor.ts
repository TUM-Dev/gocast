import { ChatMessage, ChatReaction, ChatReactionGroup } from "../api/chat";
import { TopEmojis } from "top-twitter-emojis-map";
import { EmojiPicker } from "./EmojiPicker";

export abstract class ChatMessagePreprocessor {
    static MAX_NAMES_IN_REACTION_TITLE = 2;

    static AggregateReactions(m: ChatMessage, userId: number): ChatMessage {
        m.aggregatedReactions = (m.reactions || [])
            .reduce((res: ChatReactionGroup[], reaction: ChatReaction) => {
                let group: ChatReactionGroup = res.find((r) => r.emojiName === reaction.emoji);
                if (group === undefined) {
                    group = {
                        emoji: TopEmojis.find((e) => e.short_names.includes(reaction.emoji)).emoji,
                        emojiName: reaction.emoji,
                        reactions: [],
                        names: [],
                        namesPretty: "",
                        hasReacted: reaction.userID === userId,
                    };
                    res.push(group);
                } else if (reaction.userID == userId) {
                    group.hasReacted = true;
                }

                group.names.push(reaction.username);
                group.reactions.push(reaction);
                return res;
            }, [])
            .map((group) => {
                if (group.names.length === 0) {
                    // Nobody
                    group.namesPretty = `Nobody reacted with ${group.emojiName}`;
                } else if (group.names.length == 1) {
                    // One Person
                    group.namesPretty = `${group.names[0]} reacted with ${group.emojiName}`;
                } else if (group.names.length == ChatMessagePreprocessor.MAX_NAMES_IN_REACTION_TITLE + 1) {
                    // 1 person more than max allowed
                    group.namesPretty = `${group.names
                        .slice(0, ChatMessagePreprocessor.MAX_NAMES_IN_REACTION_TITLE)
                        .join(", ")} and one other reacted with ${group.emojiName}`;
                } else if (group.names.length > ChatMessagePreprocessor.MAX_NAMES_IN_REACTION_TITLE) {
                    // at least 2 more than max allowed
                    group.namesPretty = `${group.names
                        .slice(0, ChatMessagePreprocessor.MAX_NAMES_IN_REACTION_TITLE)
                        .join(", ")} and ${
                        group.names.length - ChatMessagePreprocessor.MAX_NAMES_IN_REACTION_TITLE
                    } others reacted with ${group.emojiName}`;
                } else {
                    // More than 1 Person but less than MAX_NAMES_IN_REACTION_TITLE
                    group.namesPretty = `${group.names.slice(0, group.names.length - 1).join(", ")} and ${
                        group.names[group.names.length - 1]
                    } reacted with ${group.emojiName}`;
                }
                return group;
            });
        m.aggregatedReactions.sort(
            (a, b) => EmojiPicker.getEmojiIndex(a.emojiName) - EmojiPicker.getEmojiIndex(b.emojiName),
        );
        return m;
    }
}
