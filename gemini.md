
개요 
- 두 MongoDB 의 동일한 Collection 끼리의 Index 를 비교하여 서로 동일한지 확인한다. 
- 결과는 Collection 명과 Index 명을 나열하고 일치하는지 여부를 나타낸다. 

입력정보 
- Source 의 MongoDB 접속 정보 
- Target 의 MongoDB 접속 정보 
- Source 의 데이터베이스 명 
- Target 의 데이터베이스 명 


수행형식
- Source 접속 정보와 데이터베이스에 접근하고 전체 Collection 목록을 가지고 온다. 
- 해당 Collection 을 하나씩 순회하면서 Index 정보를 얻는다. 
- Target 접속 정보에 데이터베이스에 접근하여 동일한 Collection 에 인덱스가 일치하는지 확인한다. 
- Target 쪽에 Index 가 없는 경우 해당 인덱스의 생성 구문을 화면에 출력한다. 
- force-create-index 옵션이 있다면 Target 쪽에 인덱스를 삭제하고 Source 에 있는 것을 내용으로 재생성을 수행한다. 
