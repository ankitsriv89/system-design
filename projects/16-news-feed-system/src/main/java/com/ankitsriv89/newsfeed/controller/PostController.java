package com.ankitsriv89.newsfeed.controller;

import com.ankitsriv89.newsfeed.domain.Post;
import com.ankitsriv89.newsfeed.dto.CreatePostRequest;
import com.ankitsriv89.newsfeed.dto.PostResponse;
import com.ankitsriv89.newsfeed.service.PostService;
import jakarta.validation.Valid;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/v1/posts")
public class PostController {

    private final PostService postService;

    public PostController(PostService postService) {
        this.postService = postService;
    }

    private static String currentUser() {
        return (String) SecurityContextHolder.getContext().getAuthentication().getPrincipal();
    }

    @PostMapping
    public ResponseEntity<PostResponse> create(@Valid @RequestBody CreatePostRequest req) {
        Post post = postService.create(currentUser(), req.body().trim());
        return ResponseEntity.status(HttpStatus.CREATED).body(PostResponse.from(post));
    }

    @DeleteMapping("/{postId}")
    public ResponseEntity<Void> delete(@PathVariable long postId) {
        postService.delete(currentUser(), postId);
        return ResponseEntity.noContent().build();
    }
}
