import { TestBed } from '@angular/core/testing';

import { WatchService } from './watch.service';

describe('WatchService', () => {
  let service: WatchService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(WatchService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });
});
