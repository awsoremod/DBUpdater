create table problems (                                                                            
  id bigint primary key,                                                                           
  contest_id bigint,                                                                               
  code varchar not null,                                                                           
  constraint fk_problems_contest_id foreign key (contest_id) references contests (id)              
);                                                                                                 
                                                                                                   
create unique index on problems (contest_id, code);                                                
                                                                                                   
create table submissions (                                                                         
  id bigint primary key,                                                                           
  user_id bigint,                                                                                  
  problem_id bigint,                                                                               
  success boolean not null,                                                                        
  submitted_at timestamp not null,                                                                 
  constraint fk_submissions_user_id foreign key (user_id) references users (id),                   
  constraint fk_submissions_problem_id foreign key (problem_id) references problems (id)           
);