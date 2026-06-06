package com.ankitsriv89.twitter.dto;

/** One trending hashtag and how many tweets used it in the trend window. */
public record Trend(
        String hashtag,
        long count
) {
}
