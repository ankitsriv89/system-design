package com.ankitsriv89.twitter.api;

import com.ankitsriv89.twitter.domain.Tweet;
import com.ankitsriv89.twitter.dto.CreateTweetRequest;
import com.ankitsriv89.twitter.dto.TweetResponse;
import com.ankitsriv89.twitter.service.TweetService;
import jakarta.validation.Valid;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/v1/posts")
public class TweetController {

    private final TweetService tweetService;

    public TweetController(TweetService tweetService) {
        this.tweetService = tweetService;
    }

    private static String currentUser() {
        return (String) SecurityContextHolder.getContext().getAuthentication().getPrincipal();
    }

    @PostMapping
    public ResponseEntity<TweetResponse> create(@Valid @RequestBody CreateTweetRequest req) {
        Tweet tweet = tweetService.create(currentUser(), req.text().trim());
        return ResponseEntity.status(HttpStatus.CREATED).body(TweetResponse.from(tweet));
    }

    @DeleteMapping("/{tweetId}")
    public ResponseEntity<Void> delete(@PathVariable long tweetId) {
        tweetService.delete(currentUser(), tweetId);
        return ResponseEntity.noContent().build();
    }
}
